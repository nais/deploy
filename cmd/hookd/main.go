package main

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/Shopify/sarama"
	"github.com/go-chi/chi"
	chi_middleware "github.com/go-chi/chi/middleware"
	"github.com/golang/protobuf/proto"
	gh "github.com/google/go-github/v27/github"
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/common/pkg/kafka"
	"github.com/navikt/deployment/common/pkg/logging"
	"github.com/navikt/deployment/hookd/pkg/api/v1/apikey"
	"github.com/navikt/deployment/hookd/pkg/api/v1/deploy"
	"github.com/navikt/deployment/hookd/pkg/api/v1/provision"
	api_v1_queue "github.com/navikt/deployment/hookd/pkg/api/v1/queue"
	"github.com/navikt/deployment/hookd/pkg/api/v1/status"
	"github.com/navikt/deployment/hookd/pkg/api/v1/teams"
	"github.com/navikt/deployment/hookd/pkg/auth"
	"github.com/navikt/deployment/hookd/pkg/azure/discovery"
	"github.com/navikt/deployment/hookd/pkg/config"
	"github.com/navikt/deployment/hookd/pkg/github"
	"github.com/navikt/deployment/hookd/pkg/logproxy"
	"github.com/navikt/deployment/hookd/pkg/metrics"
	"github.com/navikt/deployment/hookd/pkg/middleware"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	"github.com/navikt/deployment/hookd/pkg/server"
	"github.com/navikt/deployment/pkg/crypto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

var (
	cfg            = config.DefaultConfig()
	retryInterval  = time.Second * 5
	queueSize      = 32
	requestTimeout = time.Second * 10
)

func init() {
	flag.BoolVar(&cfg.Github.Enabled, "github-enabled", cfg.Github.Enabled, "Enable connections to Github.")
	flag.StringVar(&cfg.Github.WebhookSecret, "github-webhook-secret", cfg.Github.WebhookSecret, "Github pre-shared webhook secret key.")
	flag.IntVar(&cfg.Github.ApplicationID, "github-app-id", cfg.Github.ApplicationID, "Github App ID.")
	flag.IntVar(&cfg.Github.InstallID, "github-install-id", cfg.Github.InstallID, "Github App installation ID.")
	flag.StringVar(&cfg.Github.KeyFile, "github-key-file", cfg.Github.KeyFile, "Path to PEM key owned by Github App.")
	flag.StringVar(&cfg.Github.ClientID, "github-client-id", cfg.Github.ClientID, "Client ID of the Github App.")
	flag.StringVar(&cfg.Github.ClientSecret, "github-client-secret", cfg.Github.ClientSecret, "Client secret of the GitHub App.")

	flag.StringVar(&cfg.BaseURL, "base-url", cfg.BaseURL, "Base URL where hookd can be reached.")
	flag.StringVar(&cfg.ListenAddress, "listen-address", cfg.ListenAddress, "IP:PORT")
	flag.StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "Log format, either 'json' or 'text'.")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Logging verbosity level.")
	flag.StringSliceVar(&cfg.Clusters, "clusters", cfg.Clusters, "Comma-separated list of valid clusters that can be deployed to.")
	flag.StringVar(&cfg.ProvisionKey, "provision-key", cfg.ProvisionKey, "Pre-shared key for /api/v1/provision endpoint.")
	flag.StringVar(&cfg.EncryptionKey, "encryption-key", cfg.EncryptionKey, "Pre-shared key used for message encryption over Kafka.")

	flag.StringVar(&cfg.S3.Endpoint, "s3-endpoint", cfg.S3.Endpoint, "S3 endpoint for state storage.")
	flag.StringVar(&cfg.S3.AccessKey, "s3-access-key", cfg.S3.AccessKey, "S3 access key.")
	flag.StringVar(&cfg.S3.SecretKey, "s3-secret-key", cfg.S3.SecretKey, "S3 secret key.")
	flag.StringVar(&cfg.S3.BucketName, "s3-bucket-name", cfg.S3.BucketName, "S3 bucket name.")
	flag.StringVar(&cfg.S3.BucketLocation, "s3-bucket-location", cfg.S3.BucketLocation, "S3 bucket location.")
	flag.BoolVar(&cfg.S3.UseTLS, "s3-secure", cfg.S3.UseTLS, "Use TLS for S3 connections.")

	flag.StringVar(&cfg.Postgres, "postgres-connection-string", cfg.Postgres, "PostgreSQL connection string.")

	flag.StringVar(&cfg.Azure.ClientID, "azure.clientid", cfg.Azure.ClientID, "Azure ClientId.")
	flag.StringVar(&cfg.Azure.ClientSecret, "azure.clientsecret", cfg.Azure.ClientSecret, "Azure ClientSecret")
	flag.StringVar(&cfg.Azure.DiscoveryURL, "azure.discoveryurl", cfg.Azure.DiscoveryURL, "Azure DiscoveryURL")
	flag.StringVar(&cfg.Azure.Tenant, "azure.tenant", cfg.Azure.Tenant, "Azure Tenant")

	kafka.SetupFlags(&cfg.Kafka)
}

func run() error {
	flag.Parse()
	certificates, err := discovery.FetchCertificates(cfg.Azure)

	if err := logging.Setup(cfg.LogLevel, cfg.LogFormat); err != nil {
		return err
	}

	kafkaLogger, err := logging.New(cfg.Kafka.Verbosity, cfg.LogFormat)
	if err != nil {
		return err
	}

	log.Info("hookd is starting")
	log.Infof("kafka topic for requests: %s", cfg.Kafka.RequestTopic)
	log.Infof("kafka topic for statuses: %s", cfg.Kafka.StatusTopic)
	log.Infof("kafka consumer group....: %s", cfg.Kafka.GroupID)
	log.Infof("kafka brokers...........: %+v", cfg.Kafka.Brokers)
	log.Infof("web frontend templates..: %s", auth.TemplateLocation)

	sarama.Logger = kafkaLogger

	if cfg.Github.Enabled && (cfg.Github.ApplicationID == 0 || cfg.Github.InstallID == 0) {
		return fmt.Errorf("--github-install-id and --github-app-id must be specified when --github-enabled=true")
	}

	provisionKey, err := hex.DecodeString(cfg.ProvisionKey)
	if err != nil {
		return fmt.Errorf("provisioning pre-shared key must be a hex encoded string")
	}

	encryptionKey, err := crypto.KeyFromHexString(cfg.EncryptionKey)
	if err != nil {
		return err
	}

	teamRepositoryStorage, err := persistence.NewS3StorageBackend(cfg.S3)
	if err != nil {
		return fmt.Errorf("while setting up S3 backend: %s", err)
	}

	kafkaClient, err := kafka.NewDualClient(
		cfg.Kafka,
		cfg.Kafka.StatusTopic,
		cfg.Kafka.RequestTopic,
	)
	if err != nil {
		return fmt.Errorf("while setting up Kafka: %s", err)
	}

	go kafkaClient.ConsumerLoop()

	var installationClient *gh.Client
	var githubClient github.Client

	if cfg.Github.Enabled {
		installationClient, err = github.InstallationClient(cfg.Github.ApplicationID, cfg.Github.InstallID, cfg.Github.KeyFile)
		if err != nil {
			return fmt.Errorf("cannot instantiate Github installation client: %s", err)
		}
		githubClient = github.New(installationClient)
	} else {
		githubClient = github.FakeClient()
	}

	apiKeys := &persistence.PostgresApiKeyStorage{
		ConnectionString: cfg.Postgres,
	}

	prometheusMiddleware := middleware.PrometheusMiddleware("hookd")

	requestChan := make(chan deployment.DeploymentRequest, queueSize)
	statusChan := make(chan deployment.DeploymentStatus, queueSize)

	deploymentHandler := &api_v1_deploy.DeploymentHandler{
		BaseURL:           cfg.BaseURL,
		DeploymentRequest: requestChan,
		DeploymentStatus:  statusChan,
		GithubClient:      githubClient,
		APIKeyStorage:     apiKeys,
		Clusters:          cfg.Clusters,
	}

	teamsHandler := &api_v1_teams.TeamsHandler{}
	apikeyHandler := &api_v1_apikey.ApiKeyHandler{
		APIKeyStorage: apiKeys,
	}


	statusHandler := &api_v1_status.StatusHandler{
		GithubClient:  githubClient,
		APIKeyStorage: apiKeys,
	}

	provisionHandler := &api_v1_provision.Handler{
		APIKeyStorage: apiKeys,
		SecretKey:     provisionKey,
	}

	githubDeploymentHandler := &server.GithubDeploymentHandler{
		DeploymentRequest:     requestChan,
		DeploymentStatus:      statusChan,
		SecretToken:           cfg.Github.WebhookSecret,
		TeamRepositoryStorage: teamRepositoryStorage,
		Clusters:              cfg.Clusters,
	}

	queueHandler := &api_v1_queue.Handler{}

	// Pre-populate request metrics
	for _, code := range api_v1_deploy.StatusCodes {
		prometheusMiddleware.Initialize("/api/v1/deploy", http.MethodPost, code)
	}
	for _, code := range api_v1_status.StatusCodes {
		prometheusMiddleware.Initialize("/api/v1/status", http.MethodPost, code)
	}
	for _, code := range api_v1_provision.StatusCodes {
		prometheusMiddleware.Initialize("/api/v1/provision", http.MethodPost, code)
	}

	// Base settings for all requests
	router := chi.NewRouter()
	router.Use(
		middleware.RequestLogger(),
		prometheusMiddleware.Handler(),
		chi_middleware.StripSlashes,
	)

	// Mount /metrics endpoint with no authentication
	router.Get(cfg.MetricsPath, promhttp.Handler().ServeHTTP)

	// Deployment logs accessible via shorthand URL
	router.HandleFunc("/logs", logproxy.HandleFunc)

	// Mount /api/v1 for API requests
	// Only application/json content type allowed
	router.Route("/api/v1", func(r chi.Router) {
		r.Use(
			chi_middleware.AllowContentType("application/json"),
			chi_middleware.Timeout(requestTimeout),
		)
		r.Route("/apikey", func(r chi.Router) {
			r.Use(
				middleware.TokenValidatorMiddleware(certificates),
			)
			r.Get("/", apikeyHandler.GetApiKeys)              // -> apikey til alle teams brukeren er autorisert for å se
			r.Get("/{team}", apikeyHandler.GetTeamApiKey)     // -> apikey til dette spesifikke teamet
			r.Post("/{team}", apikeyHandler.RotateTeamApiKey) // -> rotate key (Validere at brukeren er owner av gruppa som eier keyen)
		})
		r.Route("/teams", func(r chi.Router) {
			r.Get("/", teamsHandler.ServeHTTP) // -> ID og navn (Liste over teams brukeren har tilgang til)
		})
		r.Post("/deploy", deploymentHandler.ServeHTTP)
		r.Post("/status", statusHandler.ServeHTTP)
		r.Get("/queue", queueHandler.ServeHTTP)
		if len(provisionKey) == 0 {
			log.Error("Refusing to set up team API provisioning endpoint without pre-shared secret; try using --provision-key")
			log.Error("Note: /api/v1/provision will be unavailable")
		} else {
			r.Post("/provision", provisionHandler.ServeHTTP)
		}
	})
	// Mount /events for "legacy" GitHub deployment handling
	router.Post("/events", githubDeploymentHandler.ServeHTTP)

	// "Legacy" user authentication and repository/team connections
	router.Route("/auth", func(r chi.Router) {
		loginHandler := &auth.LoginHandler{
			ClientID: cfg.Github.ClientID,
		}
		logoutHandler := &auth.LogoutHandler{}
		callbackHandler := &auth.CallbackHandler{
			ClientID:     cfg.Github.ClientID,
			ClientSecret: cfg.Github.ClientSecret,
		}
		formHandler := &auth.FormHandler{}
		submittedFormHandler := &auth.SubmittedFormHandler{
			TeamRepositoryStorage: teamRepositoryStorage,
			ApplicationClient:     installationClient,
		}

		r.Get("/login", loginHandler.ServeHTTP)
		r.Get("/logout", logoutHandler.ServeHTTP)
		r.Get("/callback", callbackHandler.ServeHTTP)
		r.Get("/form", formHandler.ServeHTTP)
		r.Post("/submit", submittedFormHandler.ServeHTTP)

	})

	// "Legacy" proxy/caching layer between user, application, and GitHub.
	router.Route("/proxy", func(r chi.Router) {
		teamProxyHandler := &auth.TeamsProxyHandler{
			ApplicationClient: installationClient,
		}
		repositoryProxyHandler := &auth.RepositoriesProxyHandler{}

		r.Get("/teams", teamProxyHandler.ServeHTTP)
		r.Get("/repositories", repositoryProxyHandler.ServeHTTP)
	})

	// "Legacy" static assets (css, js, images)
	staticHandler := http.StripPrefix(
		"/assets",
		http.FileServer(http.Dir(auth.StaticAssetsLocation)),
	)
	router.Handle("/assets/*", staticHandler)

	go func() {
		err := http.ListenAndServe(cfg.ListenAddress, router)
		if err != nil {
			log.Error(err)
		}
	}()

	log.Infof("Ready to accept connections")

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	// Three loops:
	//
	//   1) Listen for deployment status messages from Kafka. Forward them to the
	//      deployment status queue.
	//
	//   2) Process the deployment request queue.
	//      Requests are put on the Kafka queue. Failed messages are put on the queue again.
	//
	//   3) Process the deployment status queue.
	//      Statuses are posted to Github. Failed messages are put on the queue again.
	//
	for {
		select {
		case m := <-kafkaClient.RecvQ:
			metrics.KafkaQueueSize.Set(float64(len(kafkaClient.RecvQ)))

			status := deployment.DeploymentStatus{}
			logger := kafka.ConsumerMessageLogger(&m)

			payload, err := crypto.Decrypt(m.Value, encryptionKey)
			if err != nil {
				logger.Errorf("Unable to decrypt Kafka message: %s", err)
				kafkaClient.Consumer.MarkOffset(&m, "")
				continue
			}

			err = proto.Unmarshal(payload, &status)
			if err != nil {
				logger.Errorf("Discarding incoming message: %s", err)
				kafkaClient.Consumer.MarkOffset(&m, "")
				continue
			}

			statusChan <- status
			kafkaClient.Consumer.MarkOffset(&m, "")

		case req := <-requestChan:
			metrics.DeploymentRequestQueueSize.Set(float64(len(requestChan)))

			logger := log.WithFields(req.LogFields())

			payload, err := proto.Marshal(&req)
			if err != nil {
				logger.Errorf("Marshal JSON for Kafka message: %s", err)
				continue
			}

			ciphertext, err := crypto.Encrypt(payload, encryptionKey)
			if err != nil {
				logger.Errorf("Unable to encrypt Kafka message: %s", err)
				continue
			}

			msg := sarama.ProducerMessage{
				Topic:     kafkaClient.ProducerTopic,
				Value:     sarama.StringEncoder(ciphertext),
				Timestamp: time.Unix(req.GetTimestamp(), 0),
			}

			_, _, err = kafkaClient.Producer.SendMessage(&msg)
			if err == nil {
				metrics.Dispatched.Inc()
				logger.Info("Deployment request published to Kafka")
				st := deployment.NewQueuedStatus(req)
				statusChan <- *st
				continue
			}

			logger.Errorf("Publishing message to Kafka: %s", err)
			go func() {
				logger.Tracef("Retrying in %.0f seconds", retryInterval.Seconds())
				time.Sleep(retryInterval)
				requestChan <- req
				logger.Tracef("Request resubmitted to queue")
			}()

		case status := <-statusChan:
			metrics.GithubStatusQueueSize.Set(float64(len(statusChan)))
			metrics.UpdateQueue(status)

			logger := log.WithFields(status.LogFields())

			if !cfg.Github.Enabled {
				logger.Warn("Process deployment status: discarding message due to GitHub being disabled")
				metrics.DeploymentStatus(status, 0)
				continue
			}

			ghs, req, err := github.CreateDeploymentStatus(installationClient, &status, cfg.BaseURL)
			metrics.DeploymentStatus(status, req.StatusCode)

			if err == nil {
				logger = logger.WithFields(log.Fields{
					deployment.LogFieldDeploymentStatusID: ghs.GetID(),
				})
				logger.Infof("Published deployment status to GitHub: %s", status.GetDescription())
				continue
			}

			logger.Errorf("Sending deployment status to Github: %s", err)

			if err == github.ErrEmptyRepository || err == github.ErrEmptyDeployment {
				logger.Tracef("Error is non-retriable; giving up")
				continue
			}

			go func() {
				logger.Tracef("Retrying in %.0f seconds", retryInterval.Seconds())
				time.Sleep(retryInterval)
				statusChan <- status
				logger.Tracef("Deployment status resubmitted to queue")
			}()

		case <-signals:
			return nil
		}
	}
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
	}
}
