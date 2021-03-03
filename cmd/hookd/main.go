package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	unauthenticated_interceptor "github.com/navikt/deployment/pkg/grpc/interceptor/unauthenticated"
	"net"
	"net/http"
	"os"
	"os/signal"

	"github.com/nais/liberator/pkg/conftools"
	"github.com/navikt/deployment/pkg/azure/oauth2"
	"github.com/navikt/deployment/pkg/grpc/deployserver"
	apikey_interceptor "github.com/navikt/deployment/pkg/grpc/interceptor/apikey"
	switch_interceptor "github.com/navikt/deployment/pkg/grpc/interceptor/switch"
	"github.com/navikt/deployment/pkg/grpc/interceptor/token"

	gh "github.com/google/go-github/v27/github"
	"github.com/navikt/deployment/pkg/azure/discovery"
	"github.com/navikt/deployment/pkg/azure/graphapi"
	"github.com/navikt/deployment/pkg/grpc/dispatchserver"
	"github.com/navikt/deployment/pkg/hookd/api"
	"github.com/navikt/deployment/pkg/hookd/config"
	"github.com/navikt/deployment/pkg/hookd/database"
	"github.com/navikt/deployment/pkg/hookd/github"
	"github.com/navikt/deployment/pkg/hookd/middleware"
	"github.com/navikt/deployment/pkg/logging"
	"github.com/navikt/deployment/pkg/pb"
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
	cfg := config.Initialize()
	err := conftools.Load(cfg)
	if err != nil {
		return err
	}

	if err := logging.Setup(cfg.LogLevel, cfg.LogFormat); err != nil {
		return err
	}

	log.Info("hookd starting up")

	for _, line := range conftools.Format(maskedConfig) {
		log.Info(line)
	}

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
	dispatchServer, err := startGrpcServer(*cfg, db, db, githubClient, certificates)
	if err != nil {
		return err
	}

	log.Infof("gRPC server started")

	router := api.New(api.Config{
		ApiKeyStore:                 db,
		BaseURL:                     cfg.BaseURL,
		Certificates:                certificates,
		DeploymentStore:             db,
		DispatchServer:              dispatchServer,
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

func startGrpcServer(cfg config.Config, db database.DeploymentStore, apikeys database.ApiKeyStore, githubClient github.Client, certificates map[string]discovery.CertificateList) (dispatchserver.DispatchServer, error) {
	dispatchServer := dispatchserver.New(db, githubClient)
	deployServer := deployserver.New(dispatchServer, db)

	serverOpts := make([]grpc.ServerOption, 0)
	interceptor := switch_interceptor.NewServerInterceptor()

	unauthenticatedInterceptor := &unauthenticated_interceptor.ServerInterceptor{}
	interceptor.Add(pb.Deploy_ServiceDesc.ServiceName, unauthenticatedInterceptor)
	interceptor.Add(pb.Dispatch_ServiceDesc.ServiceName, unauthenticatedInterceptor)

	if cfg.CliAuthentication {
		apikeyInterceptor := &apikey_interceptor.ServerInterceptor{
			APIKeyStore: apikeys,
		}
		interceptor.Add(pb.Deploy_ServiceDesc.ServiceName, apikeyInterceptor)
	}

	if cfg.DeploydAuthentication {
		preAuthApps := make([]oauth2.PreAuthorizedApplication, 0)
		err := json.Unmarshal([]byte(cfg.Azure.PreAuthorizedApps), &preAuthApps)
		if err != nil {
			return nil, fmt.Errorf("unmarshalling pre-authorized apps: %s", err)
		}

		tokenInterceptor := &token_interceptor.ServerInterceptor{
			Audience:     cfg.Azure.ClientID,
			Certificates: certificates,
			PreAuthApps:  preAuthApps,
		}

		interceptor.Add(pb.Dispatch_ServiceDesc.ServiceName, tokenInterceptor)

	}
	if cfg.CliAuthentication ||cfg.DeploydAuthentication {
		serverOpts = append(
			serverOpts,
			grpc.UnaryInterceptor(interceptor.UnaryServerInterceptor),
			grpc.StreamInterceptor(interceptor.StreamServerInterceptor),
		)

	}

	grpcServer := grpc.NewServer(serverOpts...)

	pb.RegisterDispatchServer(grpcServer, dispatchServer)
	pb.RegisterDeployServer(grpcServer, deployServer)

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

	return dispatchServer, nil
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
	}
}
