package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/nais/deploy/pkg/deployclient"
	"github.com/nais/deploy/pkg/pb"
	"github.com/nais/deploy/pkg/telemetry"
	"github.com/nais/deploy/pkg/version"
	"go.opentelemetry.io/otel/attribute"
	otrace "go.opentelemetry.io/otel/trace"
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
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// Logging
	deployclient.SetupLogging(*cfg)

	// Welcome
	log.Infof("NAIS deploy %s", version.Version())

	err := cfg.Validate()
	if err != nil {
		if !errors.Is(err, deployclient.ErrInvalidTelemetryFormat) {
			if !cfg.DryRun {
				return deployclient.ErrorWrap(deployclient.ExitInvocationFailure, err)
			}
			log.Warnf("Configuration did not pass validation: %s", err)
		} else {
			log.Warnf("Telemetry configuration did not pass validation: %s", err)
		}
	}

	// OpenTelemetry
	tracerProvider, err := telemetry.New(ctx, "deploy", cfg.OpenTelemetryCollectorURL)
	if err != nil {
		return fmt.Errorf("Setup OpenTelemetry: %w", err)
	}

	// Clean shutdown for OT
	defer func() {
		err := tracerProvider.Shutdown(ctx)
		if err != nil {
			log.Errorf("Shutdown OpenTelemetry: %s", err)
		}
	}()

	// Inherit traceparent from pipeline, if any.
	// If TRACEPARENT is set, ignore the TELEMETRY value.
	// If not, start a new top-level trace using the TELEMETRY variable.
	var span otrace.Span
	if len(cfg.Traceparent) > 0 {
		log.Infof("Using traceparent header %s", cfg.Traceparent)
		ctx = telemetry.WithTraceParent(ctx, cfg.Traceparent)
	} else if cfg.Telemetry != nil {
		log.Infof("Importing pipeline telemetry data as this request's top-level trace")
		ctx, span = cfg.Telemetry.StartTracing(ctx)
		defer span.End()
	} else {
		log.Infof("No top-level trace detected, starting a new one.")
	}

	// Start the deploy client's top level trace.
	ctx, span = telemetry.Tracer().Start(ctx, "NAIS deploy", otrace.WithSpanKind(otrace.SpanKindClient))
	defer span.End()

	// Print version
	ts, err := version.BuildTime()
	if err == nil {
		span.SetAttributes(attribute.KeyValue{
			Key:   "deploy.client.build-time",
			Value: attribute.StringValue(ts.Local().String()),
		})
		log.Infof("This version was built %s", ts.Local())
	}
	span.SetAttributes(attribute.KeyValue{
		Key:   "deploy.client.version",
		Value: attribute.StringValue(version.Version()),
	})

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
