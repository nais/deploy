package main

import (
	"context"
	"fmt"
	"os"

	"github.com/nais/deploy/pkg/deployclient"
	"github.com/nais/deploy/pkg/pb"
	"github.com/nais/deploy/pkg/telemetry"
	"github.com/nais/deploy/pkg/version"
	"google.golang.org/protobuf/encoding/protojson"

	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

func main() {
	err := run()
	if err == nil {
		return
	}
	code := deployclient.ErrorExitCode(err)
	if code == deployclient.ExitInvocationFailure {
		flag.Usage()
	}
	log.Errorf("fatal: %s", err)
	os.Exit(int(code))
}

func run() error {
	// Configuration and context
	cfg := deployclient.NewConfig()
	deployclient.InitConfig(cfg)
	programContext, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// Logging
	deployclient.SetupLogging(*cfg)

	// OpenTelemetry
	tracerProvider, err := telemetry.New(programContext, "deploy", cfg.OpenTelemetryCollectorURL)
	if err != nil {
		return fmt.Errorf("Setup OpenTelemetry: %w", err)
	}

	// Clean shutdown for OT
	defer func() {
		err := tracerProvider.Shutdown(programContext)
		if err != nil {
			log.Errorf("Shutdown OpenTelemetry: %s", err)
		}
	}()

	// Root span for tracing.
	// All sub-spans must be created from this context.
	ctx, rootSpan := telemetry.Tracer().Start(programContext, "Run deployment process")
	defer rootSpan.End()

	// Welcome
	log.Infof("NAIS deploy %s", version.Version())
	ts, err := version.BuildTime()
	if err == nil {
		log.Infof("This version was built %s", ts.Local())
	}

	// Prepare request
	request, err := deployclient.Prepare(ctx, cfg)
	if err != nil {
		return err
	}

	// Set up asynchronous gRPC connection
	grpcConnection, err := deployclient.NewGrpcConnection(*cfg)
	if err != nil {
		return err
	}
	defer func() {
		err := grpcConnection.Close()
		if err != nil {
			log.Error(err)
		}
	}()

	d := deployclient.Deployer{
		Client: pb.NewDeployClient(grpcConnection),
	}

	if cfg.PrintPayload {
		fmt.Println(protojson.Format(request))
	}

	if cfg.DryRun {
		return nil
	}

	return d.Deploy(ctx, cfg, request)
}
