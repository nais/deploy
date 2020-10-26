package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/Shopify/sarama"
	gh "github.com/google/go-github/v27/github"
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/common/pkg/kafka"
	"github.com/navikt/deployment/common/pkg/logging"
	"github.com/navikt/deployment/hookd/pkg/api"
	"github.com/navikt/deployment/hookd/pkg/auth"
	"github.com/navikt/deployment/hookd/pkg/azure/discovery"
	"github.com/navikt/deployment/hookd/pkg/azure/graphapi"
	"github.com/navikt/deployment/hookd/pkg/broker"
	"github.com/navikt/deployment/hookd/pkg/config"
	"github.com/navikt/deployment/hookd/pkg/database"
	"github.com/navikt/deployment/hookd/pkg/github"
	"github.com/navikt/deployment/hookd/pkg/grpc/deployserver"
	"github.com/navikt/deployment/hookd/pkg/metrics"
	"github.com/navikt/deployment/hookd/pkg/middleware"
	"github.com/navikt/deployment/pkg/crypto"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"google.golang.org/grpc"
)

var (
	cfg           = config.DefaultConfig()
	retryInterval = time.Second * 5
	queueSize     = 32
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

	flag.StringVar(&cfg.DatabaseEncryptionKey, "database-encryption-key", cfg.DatabaseEncryptionKey, "Key used to encrypt api keys at rest in PostgreSQL database.")
	flag.StringVar(&cfg.DatabaseURL, "database.url", cfg.DatabaseURL, "PostgreSQL connection information.")

	flag.StringVar(&cfg.Azure.ClientID, "azure.clientid", cfg.Azure.ClientID, "Azure ClientId.")
	flag.StringVar(&cfg.Azure.ClientSecret, "azure.clientsecret", cfg.Azure.ClientSecret, "Azure ClientSecret")
	flag.StringVar(&cfg.Azure.DiscoveryURL, "azure.discoveryurl", cfg.Azure.DiscoveryURL, "Azure DiscoveryURL")
	flag.StringVar(&cfg.Azure.Tenant, "azure.tenant", cfg.Azure.Tenant, "Azure Tenant")
	flag.StringVar(&cfg.Azure.TeamMembershipAppID, "azure.teamMembershipAppID", cfg.Azure.TeamMembershipAppID, "Application ID of canonical team list")

	kafka.SetupFlags(&cfg.Kafka)
}

func run() error {
	flag.Parse()

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

	dbEncryptionKey, err := hex.DecodeString(cfg.DatabaseEncryptionKey)
	if err != nil {
		return err
	}

	db, err := database.New(cfg.DatabaseURL, dbEncryptionKey)
	if err != nil {
		return fmt.Errorf("setup postgres connection: %s", err)
	}

	err = db.Migrate(context.Background())
	if err != nil {
		return fmt.Errorf("migrating database: %s", err)
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
		githubClient = github.New(installationClient, cfg.BaseURL)
	} else {
		githubClient = github.FakeClient()
	}

	certificates, err := discovery.FetchCertificates(cfg.Azure)
	if err != nil {
		return fmt.Errorf("unable to fetch Azure certificates: %s", err)
	}

	graphAPIClient := graphapi.NewClient(cfg.Azure)

	serializer := broker.NewSerializer(kafkaClient.ProducerTopic, encryptionKey)

	sideBrok := broker.New(db, kafkaClient.Producer, serializer, githubClient)

	// Set up gRPC server
	deployServer := &deployserver.DeployServer{}
	grpcServer := grpc.NewServer()
	deployment.RegisterDeployServer(grpcServer, deployServer)
	grpcListener, err := net.Listen("tcp", cfg.GrpcAddress)
	if err != nil {
		return fmt.Errorf("unable to set up gRPC server: %w", err)
	}
	go func() {
		err := grpcServer.Serve(grpcListener)
		if err != nil {
			log.Error(err)
			os.Exit(114)
		}
	}()

	log.Infof("gRPC server started")

	router := api.New(api.Config{
		ApiKeyStore:                 db,
		BaseURL:                     cfg.BaseURL,
		Certificates:                certificates,
		Clusters:                    cfg.Clusters,
		DeploymentStore:             db,
		Broker:                      sideBrok,
		GithubConfig:                cfg.Github,
		InstallationClient:          installationClient,
		MetricsPath:                 cfg.MetricsPath,
		OAuthKeyValidatorMiddleware: middleware.TokenValidatorMiddleware(certificates, cfg.Azure.ClientID),
		ProvisionKey:                provisionKey,
		TeamClient:                  graphAPIClient,
		TeamRepositoryStorage:       db,
	})

	go func() {
		err := http.ListenAndServe(cfg.ListenAddress, router)
		if err != nil {
			log.Error(err)
		}
	}()

	log.Infof("Ready to accept connections")

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	handleKafkaStatus := func(m sarama.ConsumerMessage) (bool, error) {
		retry := false
		status, err := serializer.Unmarshal(m)
		if err != nil {
			return retry, err
		}

		ctx, cancel := context.WithTimeout(context.Background(), retryInterval)
		defer cancel()
		err = sideBrok.HandleDeploymentStatus(ctx, *status)

		switch {
		default:
			retry = true
		case err == nil:
		case database.IsErrForeignKeyViolation(err):
		}

		return retry, err
	}

	// Loop through incoming deployment status messages from deployd and commit them to the database.
	for {
		select {
		case m := <-kafkaClient.RecvQ:
			var err error
			retry := true
			logger := kafka.ConsumerMessageLogger(&m)

			metrics.KafkaQueueSize.Set(float64(len(kafkaClient.RecvQ)))

			for retry {
				retry, err = handleKafkaStatus(m)
				if err != nil && retry {
					logger.Errorf("process deployment status: %s", err)
					time.Sleep(retryInterval)
				}
			}

			kafkaClient.Consumer.MarkOffset(&m, "")
			if err != nil {
				logger.Errorf("discard deployment status: %s", err)
			}

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
