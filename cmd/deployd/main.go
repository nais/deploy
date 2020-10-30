package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/navikt/deployment/hookd/pkg/azure/oauth2"
	"github.com/navikt/deployment/hookd/pkg/grpc/interceptor"

	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/common/pkg/logging"
	"github.com/navikt/deployment/deployd/pkg/config"
	"github.com/navikt/deployment/deployd/pkg/deployd"
	"github.com/navikt/deployment/deployd/pkg/kubeclient"
	"github.com/navikt/deployment/deployd/pkg/metrics"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

var cfg = config.DefaultConfig()

const (
	requestBackoff = 2 * time.Second
)

func init() {
	flag.StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "Log format, either 'json' or 'text'.")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Logging verbosity level.")
	flag.StringVar(&cfg.Cluster, "cluster", cfg.Cluster, "Apply changes only within this cluster.")
	flag.StringVar(&cfg.MetricsListenAddr, "metrics-listen-addr", cfg.MetricsListenAddr, "Serve metrics on this address.")
	flag.BoolVar(&cfg.GrpcUseTLS, "grpc-use-tls", cfg.GrpcUseTLS, "Use secure connection when connecting to gRPC server.")
	flag.StringVar(&cfg.GrpcServer, "grpc-server", cfg.GrpcServer, "gRPC server endpoint on hookd.")
	flag.BoolVar(&cfg.GrpcAuthentication, "grpc-authentication", cfg.GrpcAuthentication, "Use token authentication on gRPC connection.")
	flag.StringVar(&cfg.HookdApplicationID, "hookd-application-id", cfg.HookdApplicationID, "Azure application ID of hookd, used for token authentication.")
	flag.StringVar(&cfg.MetricsPath, "metrics-path", cfg.MetricsPath, "Serve metrics on this endpoint.")
	flag.BoolVar(&cfg.TeamNamespaces, "team-namespaces", cfg.TeamNamespaces, "Set to true if team service accounts live in team's own namespace.")
	flag.BoolVar(&cfg.AutoCreateServiceAccount, "auto-create-service-account", cfg.AutoCreateServiceAccount, "Set to true to automatically create service accounts.")
	flag.StringVar(&cfg.Azure.ClientID, "azure.clientid", cfg.Azure.ClientID, "Azure ClientId.")
	flag.StringVar(&cfg.Azure.ClientSecret, "azure.clientsecret", cfg.Azure.ClientSecret, "Azure ClientSecret")
	flag.StringVar(&cfg.Azure.Tenant, "azure.tenant", cfg.Azure.Tenant, "Azure Tenant")
}

func run() error {
	flag.Parse()

	if err := logging.Setup(cfg.LogLevel, cfg.LogFormat); err != nil {
		return err
	}

	log.Infof("deployd starting up")
	log.Infof("cluster.................: %s", cfg.Cluster)

	if cfg.GrpcAuthentication && len(cfg.HookdApplicationID) == 0 {
		return fmt.Errorf("authenticated gRPC calls enabled, but --hookd-application-id is not specified")
	}

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
		intercept := &interceptor.ClientInterceptor{
			Config:     tokenConfig,
			RequireTLS: cfg.GrpcUseTLS,
		}
		go intercept.TokenLoop()
		dialOptions = append(dialOptions, grpc.WithPerRPCCredentials(intercept))
	}

	dialOptions = append(dialOptions, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             20 * time.Second,
		PermitWithoutStream: true,
	}))

	grpcConnection, err := grpc.Dial(cfg.GrpcServer, dialOptions...)
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
			time.Sleep(requestBackoff)

			deploymentStream, err := grpcClient.Deployments(context.Background(), &deployment.GetDeploymentOpts{
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
