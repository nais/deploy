package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"

	"github.com/navikt/deployment/hookd/pkg/azure/oauth2"

	gh "github.com/google/go-github/v27/github"
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/common/pkg/logging"
	"github.com/navikt/deployment/hookd/pkg/api"
	"github.com/navikt/deployment/hookd/pkg/azure/discovery"
	"github.com/navikt/deployment/hookd/pkg/azure/graphapi"
	"github.com/navikt/deployment/hookd/pkg/config"
	"github.com/navikt/deployment/hookd/pkg/database"
	"github.com/navikt/deployment/hookd/pkg/github"
	"github.com/navikt/deployment/hookd/pkg/grpc/deployserver"
	"github.com/navikt/deployment/hookd/pkg/grpc/interceptor"
	"github.com/navikt/deployment/hookd/pkg/middleware"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var maskedConfig = []string{
	config.AzureClientSecret,
	config.GithubClientSecret,
	config.DatabaseEncryptionKey,
	config.DatabaseUrl,
	config.ProvisionKey,
}

func run() error {
	config.Initialize()
	cfg, err := config.New()
	if err != nil {
		return err
	}

	if err := logging.Setup(cfg.LogLevel, cfg.LogFormat); err != nil {
		return err
	}

	log.Info("hookd is starting")

	config.Print(maskedConfig)

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

	certificates := make(map[string]discovery.CertificateList)
	if cfg.Azure.HasConfig() {
		log.Infof("Azure token validation and GraphQL functionality enabled")
		certificates, err = discovery.FetchCertificates(cfg.Azure)
		if err != nil {
			return fmt.Errorf("unable to fetch Azure certificates: %s", err)
		}
	}

	graphAPIClient := graphapi.NewClient(cfg.Azure)

	// Set up gRPC server
	deployServer, err := startGrpcServer(*cfg, db, githubClient, certificates)
	if err != nil {
		return err
	}

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

func startGrpcServer(cfg config.Config, db database.DeploymentStore, githubClient github.Client, certificates map[string]discovery.CertificateList) (deployserver.DeployServer, error) {
	deployServer := deployserver.New(db, githubClient)
	serverOpts := make([]grpc.ServerOption, 0)
	if cfg.GrpcAuthentication {
		preAuthApps := make([]oauth2.PreAuthorizedApplication, 0)
		err := json.Unmarshal([]byte(cfg.Azure.PreAuthorizedApps), &preAuthApps)
		if err != nil {
			return nil, fmt.Errorf("unmarshalling pre-authorized apps: %s", err)
		}

		intercept := &interceptor.ServerInterceptor{
			Audience:     cfg.Azure.ClientID,
			Certificates: certificates,
			PreAuthApps:  preAuthApps,
		}
		serverOpts = append(
			serverOpts,
			grpc.UnaryInterceptor(intercept.UnaryServerInterceptor),
			grpc.StreamInterceptor(intercept.StreamServerInterceptor),
		)
	}
	grpcServer := grpc.NewServer(serverOpts...)
	deployment.RegisterDeployServer(grpcServer, deployServer)
	grpcListener, err := net.Listen("tcp", cfg.GrpcAddress)
	if err != nil {
		return nil, fmt.Errorf("unable to set up gRPC server: %w", err)
	}
	go func() {
		err := grpcServer.Serve(grpcListener)
		if err != nil {
			log.Error(err)
			os.Exit(114)
		}
	}()

	return deployServer, nil
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
	}
}
