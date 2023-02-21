package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-chi/chi"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	unauthenticated_interceptor "github.com/nais/deploy/pkg/grpc/interceptor/unauthenticated"
	"github.com/nais/deploy/pkg/version"
	"google.golang.org/grpc/keepalive"

	"github.com/nais/deploy/pkg/grpc/deployserver"
	apikey_interceptor "github.com/nais/deploy/pkg/grpc/interceptor/apikey"
	presharedkey_interceptor "github.com/nais/deploy/pkg/grpc/interceptor/presharedkey"
	switch_interceptor "github.com/nais/deploy/pkg/grpc/interceptor/switch"
	"github.com/nais/liberator/pkg/conftools"

	gh "github.com/google/go-github/v41/github"
	"github.com/nais/deploy/pkg/grpc/dispatchserver"
	"github.com/nais/deploy/pkg/hookd/api"
	"github.com/nais/deploy/pkg/hookd/config"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/nais/deploy/pkg/hookd/github"
	"github.com/nais/deploy/pkg/hookd/middleware"
	"github.com/nais/deploy/pkg/logging"
	"github.com/nais/deploy/pkg/pb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var maskedConfig = []string{
	config.GithubClientSecret,
	config.DatabaseEncryptionKey,
	config.DatabaseUrl,
	config.DeploydKeys,
	config.FrontendKeys,
	config.ProvisionKey,
}

const (
	databaseConnectBackoffInterval = 3 * time.Second
)

func run() error {
	var db *database.Database

	cfg := config.Initialize()
	err := conftools.Load(cfg)
	if err != nil {
		return err
	}

	if err := logging.Setup(cfg.LogLevel, cfg.LogFormat); err != nil {
		return err
	}

	// Welcome
	log.Infof("hookd %s", version.Version())
	ts, err := version.BuildTime()
	if err == nil {
		log.Infof("This version was built %s", ts.Local())
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DatabaseConnectTimeout)
	for {
		log.Infof("Connecting to database...")
		db, err = database.New(ctx, cfg.DatabaseURL, dbEncryptionKey)
		if err == nil {
			log.Infof("Database connection established.")
			break
		} else if ctx.Err() != nil {
			break
		} else {
			log.Errorf("unable to connect to database: %s", err)
			time.Sleep(databaseConnectBackoffInterval)
		}
	}
	cancel()
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
		log.Infof("Posting deployment statuses to GitHub")
		githubClient = github.New(installationClient, cfg.BaseURL)
	} else {
		githubClient = github.FakeClient()
	}

	// Set up gRPC server
	grpcServer, dispatchServer, err := startGrpcServer(*cfg, db, db, githubClient)
	if err != nil {
		return err
	}
	defer grpcServer.Stop()

	log.Infof("gRPC server started")

	var validators chi.Middlewares
	if cfg.GoogleClientId != "" && len(cfg.FrontendKeys) > 0 {
		validators = append(validators, middleware.PskValidatorMiddleware(cfg.FrontendKeys))
		log.Infof("Using PSK validator")
		googleValidator, err := middleware.NewGoogleValidator(cfg.GoogleClientId, cfg.ConsoleApiKey, cfg.ConsoleUrl, cfg.GoogleAllowedDomains)
		if err != nil {
      return fmt.Errorf("set up google validator: %w", err)
		} else {
			validators = append(validators, googleValidator.Middleware())
			log.Infof("Using GoogleValidator validator")
		}
	}

	projects, err := parseKeyVal(cfg.GoogleClusterProjects)
	if err != nil {
		return fmt.Errorf("unable to parse google cluster projects: %v", err)
	}
	router := api.New(api.Config{
		ApiKeyStore:           db,
		BaseURL:               cfg.BaseURL,
		DeploymentStore:       db,
		DispatchServer:        dispatchServer,
		GithubConfig:          cfg.Github,
		InstallationClient:    installationClient,
		MetricsPath:           cfg.MetricsPath,
		ValidatorMiddlewares:  validators,
		ProvisionKey:          provisionKey,
		TeamRepositoryStorage: db,
		Projects:              projects,
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
	sig := <-signals

	log.Infof("Received signal %s (%d), exiting...", sig, sig)

	return nil
}

func startGrpcServer(cfg config.Config, db database.DeploymentStore, apikeys database.ApiKeyStore, githubClient github.Client) (*grpc.Server, dispatchserver.DispatchServer, error) {
	dispatchServer := dispatchserver.New(db, githubClient)
	deployServer := deployserver.New(dispatchServer, db)
	unaryInterceptors := make([]grpc.UnaryServerInterceptor, 0)
	streamInterceptors := make([]grpc.StreamServerInterceptor, 0)

	serverOpts := make([]grpc.ServerOption, 0)
	unaryInterceptors = append(unaryInterceptors, grpc_prometheus.UnaryServerInterceptor)
	streamInterceptors = append(streamInterceptors, grpc_prometheus.StreamServerInterceptor)

	if cfg.GRPC.CliAuthentication || cfg.GRPC.DeploydAuthentication {
		interceptor := switch_interceptor.NewServerInterceptor()

		unauthenticatedInterceptor := &unauthenticated_interceptor.ServerInterceptor{}
		interceptor.Add(pb.Deploy_ServiceDesc.ServiceName, unauthenticatedInterceptor)
		interceptor.Add(pb.Dispatch_ServiceDesc.ServiceName, unauthenticatedInterceptor)

		if cfg.GRPC.CliAuthentication {
			apikeyInterceptor := &apikey_interceptor.ServerInterceptor{
				APIKeyStore: apikeys,
			}
			interceptor.Add(pb.Deploy_ServiceDesc.ServiceName, apikeyInterceptor)
			log.Infof("Authentication enabled for deployment requests")
		}

		if cfg.GRPC.DeploydAuthentication {
			presharedkeyInterceptor := &presharedkey_interceptor.ServerInterceptor{
				Keys: cfg.DeploydKeys,
			}

			interceptor.Add(pb.Dispatch_ServiceDesc.ServiceName, presharedkeyInterceptor)
			log.Infof("Authentication enabled for deployd connections")
		}

		unaryInterceptors = append(unaryInterceptors, interceptor.UnaryServerInterceptor)
		streamInterceptors = append(streamInterceptors, interceptor.StreamServerInterceptor)
	}

	serverOpts = append(
		serverOpts,
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
		grpc.ChainStreamInterceptor(streamInterceptors...),
	)

	serverOpts = append(serverOpts, grpc.KeepaliveParams(keepalive.ServerParameters{
		Time: cfg.GRPC.KeepaliveInterval,
	}))

	grpcServer := grpc.NewServer(serverOpts...)

	pb.RegisterDispatchServer(grpcServer, dispatchServer)
	pb.RegisterDeployServer(grpcServer, deployServer)

	grpc_prometheus.Register(grpcServer)
	grpc_prometheus.EnableHandlingTimeHistogram()

	grpcListener, err := net.Listen("tcp", cfg.GRPC.Address)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to set up gRPC server: %w", err)
	}
	go func() {
		err := grpcServer.Serve(grpcListener)
		if err != nil {
			log.Error(err)
			os.Exit(114)
		}
	}()

	return grpcServer, dispatchServer, nil
}

func parseKeyVal(projects []string) (map[string]string, error) {
	projectMap := make(map[string]string, len(projects))
	for _, pair := range projects {
		parts := strings.Split(pair, "=")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid key-value pair '%s'", pair)
		}
		projectMap[parts[0]] = parts[1]
	}
	return projectMap, nil
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
	}
}
