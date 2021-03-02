package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/nais/liberator/pkg/conftools"
	"github.com/navikt/deployment/pkg/azure/oauth2"
	"github.com/navikt/deployment/pkg/deployd/deployd"
	"github.com/navikt/deployment/pkg/deployd/operation"
	"github.com/navikt/deployment/pkg/grpc/interceptor/token"

	"github.com/navikt/deployment/pkg/deployd/config"
	"github.com/navikt/deployment/pkg/deployd/kubeclient"
	"github.com/navikt/deployment/pkg/deployd/metrics"
	"github.com/navikt/deployment/pkg/logging"
	"github.com/navikt/deployment/pkg/pb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	requestBackoff = 2 * time.Second
)

var maskedConfig = []string{
	config.AzureClientSecret,
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

	log.Infof("deployd starting up")

	for _, line := range conftools.Format(maskedConfig) {
		log.Info(line)
	}

	if cfg.GrpcAuthentication && len(cfg.HookdApplicationID) == 0 {
		return fmt.Errorf("authenticated gRPC calls enabled, but --hookd-application-id is not specified")
	}

	kube, err := kubeclient.New()
	if err != nil {
		return fmt.Errorf("cannot configure Kubernetes client: %s", err)
	}
	log.Infof("kubernetes..............: %s", kube.Config.Host)

	metricsServer := http.NewServeMux()
	metricsServer.Handle(cfg.MetricsPath, metrics.Handler())
	log.Infof("Serving metrics on %s endpoint %s", cfg.MetricsListenAddr, cfg.MetricsPath)
	go http.ListenAndServe(cfg.MetricsListenAddr, metricsServer)

	dialOptions := make([]grpc.DialOption, 0)
	if !cfg.GrpcUseTLS {
		dialOptions = append(dialOptions, grpc.WithInsecure())
	} else {
		tlsOpts := &tls.Config{}
		cred := credentials.NewTLS(tlsOpts)
		if err != nil {
			return fmt.Errorf("gRPC configured to use TLS, but system-wide CA certificate bundle cannot be loaded")
		}
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(cred))
	}

	if cfg.GrpcAuthentication {
		tokenConfig := oauth2.Config(oauth2.ClientConfig{
			ClientID:     cfg.Azure.ClientID,
			ClientSecret: cfg.Azure.ClientSecret,
			TenantID:     cfg.Azure.Tenant,
			Scopes:       []string{fmt.Sprintf("api://%s/.default", cfg.HookdApplicationID)},
		})
		intercept := &token_interceptor.ClientInterceptor{
			Config:     tokenConfig,
			RequireTLS: cfg.GrpcUseTLS,
		}
		go intercept.TokenLoop()
		dialOptions = append(dialOptions, grpc.WithPerRPCCredentials(intercept))
	}

	grpcConnection, err := grpc.Dial(cfg.GrpcServer, dialOptions...)
	if err != nil {
		return fmt.Errorf("connecting to hookd gRPC server: %s", err)
	}

	grpcClient := pb.NewDeployClient(grpcConnection)

	defer grpcConnection.Close()

	// Trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	statusChan := make(chan *pb.DeploymentStatus, 1024)
	requestChan := make(chan *pb.DeploymentRequest, 1024)

	// Keep deployment requests coming in on the request channel.
	go func() {
		for {
			time.Sleep(requestBackoff)

			deploymentStream, err := grpcClient.Deployments(context.Background(), &pb.GetDeploymentOpts{
				Cluster: cfg.Cluster,
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
		defer cancel()

		client, err := kube.TeamClient(req.GetTeam())
		if err != nil {
			statusChan <- pb.NewErrorStatus(req, err)
			return
		}

		logger := log.WithFields(req.LogFields())

		op := &operation.Operation{
			Context:    ctx,
			Logger:     logger,
			Request:    req,
			StatusChan: statusChan,
			TeamClient: client,
		}

		deployd.Run(op)
	}

	for {
		select {
		case req := <-requestChan:
			go deploy(req)

		case status := <-statusChan:
			logger := log.WithFields(status.LogFields())
			switch {
			case status == nil:
				metrics.DeployIgnored.Inc()
				break
			case status.GetState() == pb.DeploymentState_error:
				fallthrough
			case status.GetState() == pb.DeploymentState_failure:
				metrics.DeployFailed.Inc()
				logger.Errorf(status.GetMessage())
			default:
				metrics.DeploySuccessful.Inc()
				logger.Infof(status.GetMessage())
			}

			_, err = grpcClient.ReportStatus(context.Background(), status)
			if err != nil {
				logger.Errorf("While reporting deployment status: %s", err)
				statusChan <- status
				time.Sleep(5 * time.Second)
				break
			} else {
				logger.Infof("Deployment response sent successfully")
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
