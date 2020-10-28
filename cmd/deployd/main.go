package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

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

	statusChan := make(chan *deployment.DeploymentStatus, 1024)

	metricsServer := http.NewServeMux()
	metricsServer.Handle(cfg.MetricsPath, metrics.Handler())
	log.Infof("Serving metrics on %s endpoint %s", cfg.MetricsListenAddr, cfg.MetricsPath)
	go http.ListenAndServe(cfg.MetricsListenAddr, metricsServer)

	grpcConnection, err := grpc.Dial(cfg.GrpcServer, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("connecting to hookd gRPC server: %s", err)
	}

	grpcClient := deployment.NewDeployClient(grpcConnection)

	defer grpcConnection.Close()

	// Trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	go func() {
		for {
			deploymentStream, err := grpcClient.Deployments(context.Background(), &deployment.GetDeploymentOpts{
				Cluster: cfg.Cluster,
			})

			if err != nil {
				log.Errorf("Open hookd deployment stream: %s", err)
				time.Sleep(5 * time.Second)
				continue
			}

			log.Infof("Connected to hookd and receiving deployment requests")

			for {
				req, err := deploymentStream.Recv()
				if err != nil {
					log.Errorf("Receive deployment request: %v", err)
					break
				} else {
					logger := log.WithFields(req.LogFields())
					deployd.Run(logger, req, *cfg, kube, statusChan)
				}
			}

			log.Errorf("Disconnected from hookd")
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
