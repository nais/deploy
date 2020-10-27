package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/common/pkg/logging"
	"github.com/navikt/deployment/deployd/pkg/config"
	"github.com/navikt/deployment/deployd/pkg/deployd"
	"github.com/navikt/deployment/deployd/pkg/kubeclient"
	"github.com/navikt/deployment/deployd/pkg/metrics"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"google.golang.org/grpc"
)

var cfg = config.DefaultConfig()

func init() {
	flag.StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "Log format, either 'json' or 'text'.")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Logging verbosity level.")
	flag.StringVar(&cfg.Cluster, "cluster", cfg.Cluster, "Apply changes only within this cluster.")
	flag.StringVar(&cfg.MetricsListenAddr, "metrics-listen-addr", cfg.MetricsListenAddr, "Serve metrics on this address.")
	flag.StringVar(&cfg.GrpcServer, "grpc-server", cfg.GrpcServer, "gRPC server endpoint on hookd.")
	flag.StringVar(&cfg.MetricsPath, "metrics-path", cfg.MetricsPath, "Serve metrics on this endpoint.")
	flag.BoolVar(&cfg.TeamNamespaces, "team-namespaces", cfg.TeamNamespaces, "Set to true if team service accounts live in team's own namespace.")
	flag.BoolVar(&cfg.AutoCreateServiceAccount, "auto-create-service-account", cfg.AutoCreateServiceAccount, "Set to true to automatically create service accounts.")
}

func run() error {
	flag.Parse()

	if err := logging.Setup(cfg.LogLevel, cfg.LogFormat); err != nil {
		return err
	}

	log.Infof("deployd starting up")
	log.Infof("cluster.................: %s", cfg.Cluster)

	kube, err := kubeclient.New()
	if err != nil {
		return fmt.Errorf("cannot configure Kubernetes client: %s", err)
	}
	log.Infof("kubernetes..............: %s", kube.Config.Host)

	grpcConnection, err := grpc.Dial(cfg.GrpcServer, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("connecting to hookd gRPC server: %w", err)
	}

	grpcClient := deployment.NewDeployClient(grpcConnection)
	deploymentStream, err := grpcClient.Deployments(context.Background(), &deployment.GetDeploymentOpts{
		Cluster: cfg.Cluster,
	})
	if err != nil {
		return fmt.Errorf("open deployment stream: %w", err)
	}

	statusChan := make(chan *deployment.DeploymentStatus, 1024)

	metricsServer := http.NewServeMux()
	metricsServer.Handle(cfg.MetricsPath, metrics.Handler())
	log.Infof("Serving metrics on %s endpoint %s", cfg.MetricsListenAddr, cfg.MetricsPath)
	go http.ListenAndServe(cfg.MetricsListenAddr, metricsServer)

	// Trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	go func() {
		for {
			req, err := deploymentStream.Recv()
			if err != nil {
				log.Errorf("get next deployment: %v", err)
			} else {
				logger := log.WithFields(req.LogFields())
				deployd.Run(logger, req, *cfg, kube, statusChan)
			}
		}
	}()

	for {
	SEL:
		select {
		case status := <-statusChan:
			logger := log.WithFields(status.LogFields())
			switch {
			case status == nil:
				metrics.DeployIgnored.Inc()
				break SEL
			case status.GetState() == deployment.GithubDeploymentState_error:
				fallthrough
			case status.GetState() == deployment.GithubDeploymentState_failure:
				metrics.DeployFailed.Inc()
				logger.Errorf(status.GetDescription())
			default:
				metrics.DeploySuccessful.Inc()
				logger.Infof(status.GetDescription())
			}

			_, err = grpcClient.ReportStatus(context.Background(), status)
			if err != nil {
				logger.Errorf("While reporting deployment status: %s", err)
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
