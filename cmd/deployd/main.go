package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nais/liberator/pkg/conftools"
	log "github.com/sirupsen/logrus"
	ocodes "go.opentelemetry.io/otel/codes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	"github.com/nais/deploy/pkg/deployd/config"
	"github.com/nais/deploy/pkg/deployd/deployd"
	"github.com/nais/deploy/pkg/deployd/kubeclient"
	"github.com/nais/deploy/pkg/deployd/metrics"
	"github.com/nais/deploy/pkg/deployd/operation"
	presharedkey_interceptor "github.com/nais/deploy/pkg/grpc/interceptor/presharedkey"
	"github.com/nais/deploy/pkg/logging"
	"github.com/nais/deploy/pkg/pb"
	"github.com/nais/deploy/pkg/telemetry"
	"github.com/nais/deploy/pkg/version"
	otrace "go.opentelemetry.io/otel/trace"
)

const (
	requestBackoff            = 2 * time.Second
	statusQueueReportInterval = 5 * time.Second
)

var maskedConfig = []string{
	config.HookdKey,
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

	// Welcome
	log.Infof("deployd %s", version.Version())
	ts, err := version.BuildTime()
	if err == nil {
		log.Infof("This version was built %s", ts.Local())
	}

	programContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	// OpenTelemetry
	tracerProvider, err := telemetry.New(programContext, "deployd", cfg.OpenTelemetryCollectorURL)
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

	for _, line := range conftools.Format(maskedConfig) {
		log.Info(line)
	}

	if cfg.GRPC.Authentication && len(cfg.HookdKey) == 0 {
		return fmt.Errorf("authenticated gRPC calls enabled, but --hookd-key is not specified")
	}

	kube, err := kubeclient.DefaultClient()
	if err != nil {
		return fmt.Errorf("cannot configure Kubernetes client: %s", err)
	}

	metricsServer := http.NewServeMux()
	metricsServer.Handle(cfg.MetricsPath, metrics.Handler())
	log.Infof("Serving metrics on %s endpoint %s", cfg.MetricsListenAddr, cfg.MetricsPath)
	go http.ListenAndServe(cfg.MetricsListenAddr, metricsServer)

	dialOptions := make([]grpc.DialOption, 0)
	if !cfg.GRPC.UseTLS {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsOpts := &tls.Config{}
		cred := credentials.NewTLS(tlsOpts)
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(cred))
	}

	if cfg.GRPC.Authentication {
		intercept := &presharedkey_interceptor.ClientInterceptor{
			RequireTLS: cfg.GRPC.UseTLS,
			Key:        cfg.HookdKey,
		}
		dialOptions = append(dialOptions, grpc.WithPerRPCCredentials(intercept))

		// Client-side parameters should be kept in sync with the server-side settings to avoid throttling (GOAWAY/ENHANCE_YOUR_CALM).
		dialOptions = append(dialOptions, grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			PermitWithoutStream: true,
		}))
	}

	grpcConnection, err := grpc.Dial(cfg.GRPC.Server, dialOptions...)
	if err != nil {
		return fmt.Errorf("connecting to hookd gRPC server: %s", err)
	}

	grpcClient := pb.NewDispatchClient(grpcConnection)

	defer grpcConnection.Close()

	// Trap SIGINT and SIGTERM to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	startupTime := time.Now()
	statusChan := make(chan *pb.DeploymentStatus, 1024)
	requestChan := make(chan *pb.DeploymentRequest, 1024)

	// Keep deployment requests coming in on the request channel.
	go func() {
		for {
			time.Sleep(requestBackoff)

			deploymentStream, err := grpcClient.Deployments(programContext, &pb.GetDeploymentOpts{
				Cluster:     cfg.Cluster,
				StartupTime: pb.TimeAsTimestamp(startupTime),
			})
			if err != nil {
				log.Errorf("Open hookd deployment stream: %s", err)
				continue
			}

			log.Infof("Connected to hookd and receiving deployment requests")

			for {
				req, err := deploymentStream.Recv()
				if err != nil {
					log.Errorf("Receive deployment request: %v", err)
					break
				}
				requestChan <- req
			}

			log.Errorf("Disconnected from hookd")
		}
	}()

	deploy := func(req *pb.DeploymentRequest) {
		ctx, cancel := req.Context()
		ctx = telemetry.WithTraceParent(ctx, req.TraceParent)
		ctx, span := telemetry.Tracer().Start(ctx, "Deploy to Kubernetes", otrace.WithSpanKind(otrace.SpanKindServer))

		client, err := kube.Impersonate(req.GetTeam())
		if err != nil {
			span.SetStatus(ocodes.Error, err.Error())
			span.End()
			cancel()
			statusChan <- pb.NewErrorStatus(req, err)
			return
		}

		logger := log.WithFields(req.LogFields())

		op := &operation.Operation{
			Context:    ctx,
			Cancel:     cancel,
			Logger:     logger,
			Request:    req,
			Trace:      span,
			StatusChan: statusChan,
		}

		deployd.Run(op, client)
	}

	statusQueue := make([]*pb.DeploymentStatus, 0, 128)

	report := func(st *pb.DeploymentStatus) error {
		logger := log.WithFields(st.LogFields())
		switch {
		case st == nil:
			metrics.DeployIgnored.Inc()
		case st.GetState() == pb.DeploymentState_error:
			fallthrough
		case st.GetState() == pb.DeploymentState_failure:
			metrics.DeployFailed.Inc()
			logger.Errorf(st.GetMessage())
		default:
			metrics.DeploySuccessful.Inc()
			logger.Infof(st.GetMessage())
		}

		_, err = grpcClient.ReportStatus(programContext, st)

		return err
	}

	reportAllInQueue := func() {
		for i, st := range statusQueue {
			err := report(st)
			if err != nil {
				logger := log.WithFields(st.LogFields())
				logger.Error(err)
				switch status.Convert(err).Code() {
				case codes.FailedPrecondition, codes.InvalidArgument, codes.AlreadyExists:
					// drop message on terminal error conditions
					logger.Warnf("Dropping message because server did not accept it: %s", st.GetMessage())
				default:
					// re-queue on all other error conditions
					statusQueue = statusQueue[i:]
					return
				}
			}
		}
		statusQueue = statusQueue[:0]
	}

	for {
		select {
		case req := <-requestChan:
			go deploy(req)

		case st := <-statusChan:
			statusQueue = append(statusQueue, st)
			reportAllInQueue()

		case <-time.NewTimer(statusQueueReportInterval).C:
			reportAllInQueue()

		case sig := <-signals:
			log.Infof("Received signal %s (%d), exiting...", sig, sig)
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
