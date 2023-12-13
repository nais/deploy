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
	"syscall"
	"time"

	"github.com/go-chi/chi"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/nais/liberator/pkg/conftools"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/nais/deploy/pkg/grpc/deployserver"
	"github.com/nais/deploy/pkg/grpc/dispatchserver"
	auth_interceptor "github.com/nais/deploy/pkg/grpc/interceptor/auth"
	presharedkey_interceptor "github.com/nais/deploy/pkg/grpc/interceptor/presharedkey"
	switch_interceptor "github.com/nais/deploy/pkg/grpc/interceptor/switch"
	unauthenticated_interceptor "github.com/nais/deploy/pkg/grpc/interceptor/unauthenticated"
	"github.com/nais/deploy/pkg/hookd/api"
	"github.com/nais/deploy/pkg/hookd/config"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/nais/deploy/pkg/hookd/logproxy"
	"github.com/nais/deploy/pkg/hookd/middleware"
	"github.com/nais/deploy/pkg/logging"
	"github.com/nais/deploy/pkg/pb"
	"github.com/nais/deploy/pkg/teams"
	"github.com/nais/deploy/pkg/version"
)

var maskedConfig = []string{
	config.ConsoleApiKey,
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

	// Set up gRPC server
	grpcServer, dispatchServer, err := startGrpcServer(*cfg, db, db)
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
		MetricsPath:           cfg.MetricsPath,
		ValidatorMiddlewares:  validators,
		PSKValidator:          middleware.PskValidatorMiddleware(cfg.FrontendKeys),
		ProvisionKey:          provisionKey,
		TeamRepositoryStorage: db,
		Projects:              projects,
		LogLinkFormatter:      logproxy.ParseLogLinkFormatter(cfg.LogLinkFormatter),
	})

	go func() {
		err := http.ListenAndServe(cfg.ListenAddress, router)
		if err != nil {
			log.Error(err)
		}
	}()

	log.Infof("Ready to accept connections")

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	sig := <-signals

	log.Infof("Received signal %s (%d), exiting...", sig, sig)

	return nil
}

func startGrpcServer(cfg config.Config, db database.DeploymentStore, apikeys database.ApiKeyStore) (*grpc.Server, dispatchserver.DispatchServer, error) {
	dispatchServer := dispatchserver.New(db)
	deployServer := deployserver.New(dispatchServer, db)
	unaryInterceptors := make([]grpc.UnaryServerInterceptor, 0)
	streamInterceptors := make([]grpc.StreamServerInterceptor, 0)

	serverOpts := make([]grpc.ServerOption, 0)

	serverMetrics := grpc_prometheus.NewServerMetrics(
		grpc_prometheus.WithServerHandlingTimeHistogram(),
	)
	prometheus.MustRegister(serverMetrics)

	unaryInterceptors = append(unaryInterceptors, serverMetrics.UnaryServerInterceptor())
	streamInterceptors = append(streamInterceptors, serverMetrics.StreamServerInterceptor())

	if cfg.GRPC.CliAuthentication || cfg.GRPC.DeploydAuthentication {
		interceptor := switch_interceptor.NewServerInterceptor()

		unauthenticatedInterceptor := &unauthenticated_interceptor.ServerInterceptor{}
		interceptor.Add(pb.Deploy_ServiceDesc.ServiceName, unauthenticatedInterceptor)
		interceptor.Add(pb.Dispatch_ServiceDesc.ServiceName, unauthenticatedInterceptor)

		if cfg.GRPC.CliAuthentication {
			ghValidator, err := auth_interceptor.NewGithubValidator()
			if err != nil {
				return nil, nil, fmt.Errorf("unable to set up github validator: %w", err)
			}

			authInterceptor := auth_interceptor.NewServerInterceptor(apikeys, ghValidator, teams.New(cfg.TeamsURL, cfg.TeamsAPIKey))

			interceptor.Add(pb.Deploy_ServiceDesc.ServiceName, authInterceptor)
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

	serverOpts = append(
		serverOpts,
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time: cfg.GRPC.KeepaliveInterval,
		}),
		// Server-side enforcement policy MUST match or be more lenient than client-side settings to avoid throttling (GOAWAY/ENHANCE_YOUR_CALM).
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)

	grpcServer := grpc.NewServer(serverOpts...)

	pb.RegisterDispatchServer(grpcServer, dispatchServer)
	pb.RegisterDeployServer(grpcServer, deployServer)

	serverMetrics.InitializeMetrics(grpcServer)

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
