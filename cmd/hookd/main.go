package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"

	gh "github.com/google/go-github/v27/github"
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/common/pkg/logging"
	"github.com/navikt/deployment/hookd/pkg/api"
	"github.com/navikt/deployment/hookd/pkg/auth"
	"github.com/navikt/deployment/hookd/pkg/azure/discovery"
	"github.com/navikt/deployment/hookd/pkg/azure/graphapi"
	"github.com/navikt/deployment/hookd/pkg/config"
	"github.com/navikt/deployment/hookd/pkg/database"
	"github.com/navikt/deployment/hookd/pkg/github"
	"github.com/navikt/deployment/hookd/pkg/grpc/deployserver"
	"github.com/navikt/deployment/hookd/pkg/middleware"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"google.golang.org/grpc"
)

var (
	cfg = config.DefaultConfig()
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

	flag.StringVar(&cfg.DatabaseEncryptionKey, "database-encryption-key", cfg.DatabaseEncryptionKey, "Key used to encrypt api keys at rest in PostgreSQL database.")
	flag.StringVar(&cfg.DatabaseURL, "database.url", cfg.DatabaseURL, "PostgreSQL connection information.")

	flag.StringVar(&cfg.Azure.ClientID, "azure.clientid", cfg.Azure.ClientID, "Azure ClientId.")
	flag.StringVar(&cfg.Azure.ClientSecret, "azure.clientsecret", cfg.Azure.ClientSecret, "Azure ClientSecret")
	flag.StringVar(&cfg.Azure.DiscoveryURL, "azure.discoveryurl", cfg.Azure.DiscoveryURL, "Azure DiscoveryURL")
	flag.StringVar(&cfg.Azure.Tenant, "azure.tenant", cfg.Azure.Tenant, "Azure Tenant")
	flag.StringVar(&cfg.Azure.TeamMembershipAppID, "azure.teamMembershipAppID", cfg.Azure.TeamMembershipAppID, "Application ID of canonical team list")
	flag.StringVar(&cfg.Azure.PreAuthorizedApps, "azure.preAuthorizedApps", cfg.Azure.PreAuthorizedApps, "Preauthorized Applications as Json")
}

func run() error {
	flag.Parse()

	if err := logging.Setup(cfg.LogLevel, cfg.LogFormat); err != nil {
		return err
	}

	log.Info("hookd is starting")
	log.Infof("web frontend templates..: %s", auth.TemplateLocation)

	if cfg.Github.Enabled && (cfg.Github.ApplicationID == 0 || cfg.Github.InstallID == 0) {
		return fmt.Errorf("--github-install-id and --github-app-id must be specified when --github-enabled=true")
	}

	provisionKey, err := hex.DecodeString(cfg.ProvisionKey)
	if err != nil {
		return fmt.Errorf("provisioning pre-shared key must be a hex encoded string")
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

	// Set up gRPC server
	deployServer := deployserver.New(db, githubClient)
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
		DeployServer:                deployServer,
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
	<-signals

	return nil
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
	}
}
