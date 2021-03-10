package deployer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/navikt/deployment/pkg/hookd/logproxy"
	"github.com/navikt/deployment/pkg/pb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
)

type TemplateVariables map[string]interface{}

const (
	DefaultRef           = "master"
	DefaultOwner         = "navikt"
	DefaultDeployServer  = "deploy.nais.io:443"
	DefaultDeployTimeout = time.Minute * 10

	ResourceRequiredMsg = "at least one Kubernetes resource is required to make sense of the deployment"
	APIKeyRequiredMsg   = "API key required"
	ClusterRequiredMsg  = "cluster required; see https://doc.nais.io/clusters"
	MalformedAPIKeyMsg  = "API key must be a hex encoded string"
)

type Deployer struct {
	Client pb.DeployClient
}

func Prepare(ctx context.Context, cfg *Config) (*pb.DeploymentRequest, error) {
	var err error
	var templateVariables = make(TemplateVariables)

	err = cfg.Validate()
	if err != nil {
		if !cfg.DryRun {
			return nil, ErrorWrap(ExitInvocationFailure, err)
		}

		log.Warnf("Config did not pass validation: %s", err)
	}

	if len(cfg.VariablesFile) > 0 {
		templateVariables, err = templateVariablesFromFile(cfg.VariablesFile)
		if err != nil {
			return nil, Errorf(ExitInvocationFailure, "load template variables: %s", err)
		}
	}

	if len(cfg.Variables) > 0 {
		templateOverrides := templateVariablesFromSlice(cfg.Variables)
		for key, val := range templateOverrides {
			if oldval, ok := templateVariables[key]; ok {
				log.Warnf("Overwriting template variable '%s'; previous value was '%v'", key, oldval)
			}
			log.Infof("Setting template variable '%s' to '%v'", key, val)
			templateVariables[key] = val
		}
	}

	resources := make([]json.RawMessage, 0)

	for i, path := range cfg.Resource {
		parsed, err := MultiDocumentFileAsJSON(path, templateVariables)
		if err != nil {
			if cfg.PrintPayload {
				errStr := err.Error()[len(path)+2:]
				line, er := detectErrorLine(errStr)
				if er == nil {
					ctx := errorContext(string(resources[i]), line)
					for _, l := range ctx {
						fmt.Println(l)
					}
				}
			}
			return nil, ErrorWrap(ExitTemplateError, err)
		}
		resources = append(resources, parsed...)
	}

	if len(cfg.Team) == 0 {
		log.Infof("Team not explicitly specified; attempting auto-detection...")
		for i, path := range cfg.Resource {
			team := detectTeam(resources[i])
			if len(team) > 0 {
				log.Infof("Detected team '%s' in path %s", team, path)
				cfg.Team = team
				break
			}
		}

		if len(cfg.Team) == 0 {
			return nil, Errorf(ExitInvocationFailure, "no team specified, and unable to auto-detect from nais.yaml")
		}
	}

	if len(cfg.Environment) == 0 {
		log.Infof("Environment not explicitly specified; attempting auto-detection...")

		namespaces := make(map[string]interface{})
		cfg.Environment = cfg.Cluster

		for i := range cfg.Resource {
			namespace := detectNamespace(resources[i])
			namespaces[namespace] = new(interface{})
		}

		if len(namespaces) == 1 {
			for namespace := range namespaces {
				if len(namespace) != 0 {
					cfg.Environment = fmt.Sprintf("%s:%s", cfg.Cluster, namespace)
				}
			}
		}

		log.Infof("Detected environment '%s'", cfg.Environment)
	}

	allResources, err := wrapResources(resources)
	if err != nil {
		return nil, ErrorWrap(ExitInvocationFailure, err)
	}

	kube, err := pb.KubernetesFromJSONResources(allResources)
	if err != nil {
		return nil, ErrorWrap(ExitInvocationFailure, err)
	}

	deadline, _ := ctx.Deadline()

	return MakeDeploymentRequest(*cfg, deadline, kube), nil
}

func (d *Deployer) Deploy(ctx context.Context, cfg *Config, deployRequest *pb.DeploymentRequest) error {
	var deployStatus *pb.DeploymentStatus
	var err error

	log.Infof("Sending deployment request to NAIS deploy at %s...", cfg.DeployServerURL)

	err = retryUnavailable(cfg.RetryInterval, cfg.Retry, func() error {
		deployStatus, err = d.Client.Deploy(ctx, deployRequest)
		return err
	})

	if err != nil {
		err = fmt.Errorf(formatGrpcError(err))
		if ctx.Err() != nil {
			return Errorf(ExitTimeout, "deployment timed out: %w", ctx.Err())
		}
		return ErrorWrap(ExitNoDeployment, err)
	}

	log.Infof("Deployment request accepted by NAIS deploy and dispatched to cluster '%s'.", deployStatus.GetRequest().GetCluster())

	if deployStatus.GetState().Finished() {
		logDeployStatus(deployStatus)
		return ErrorStatus(deployStatus)
	}

	if !cfg.Wait {
		logDeployStatus(deployStatus)
		return nil
	}

	deployRequest.ID = deployStatus.GetRequest().GetID()

	urlPrefix := "https://" + strings.Split(cfg.DeployServerURL, ":")[0]
	log.Infof("Deployment information")
	log.Infof("---")
	log.Infof("id...........: %s", deployRequest.GetID())
	log.Infof("debug logs...: %s", logproxy.MakeURL(urlPrefix, deployRequest.GetID(), deployRequest.GetTime().AsTime()))
	log.Infof("deadline.....: %s", deployRequest.GetDeadline().AsTime().Local())
	log.Infof("---")
	log.Infof("Waiting for deployment to complete...", deployRequest.GetDeadline().AsTime().Sub(time.Now()))

	var stream pb.Deploy_StatusClient
	var connectionLost bool

	for ctx.Err() == nil {
		err = retryUnavailable(cfg.RetryInterval, cfg.Retry, func() error {
			stream, err = d.Client.Status(ctx, deployRequest)
			if err != nil {
				connectionLost = true
			} else if connectionLost {
				log.Infof("Connection to NAIS deploy re-established.")
			}
			return err
		})
		if err != nil {
			return ErrorWrap(ExitUnavailable, err)
		}

		for ctx.Err() == nil {
			deployStatus, err = stream.Recv()
			if err != nil {
				connectionLost = true
				if cfg.Retry && grpcErrorCode(err) == codes.Unavailable {
					log.Warnf(formatGrpcError(err))
					break
				} else {
					return Errorf(ExitUnavailable, formatGrpcError(err))
				}
			}
			logDeployStatus(deployStatus)
			if deployStatus.GetState().Finished() {
				return ErrorStatus(deployStatus)
			}
		}
	}

	return Errorf(ExitTimeout, "deployment timed out: %w", ctx.Err())
}

func retryUnavailable(interval time.Duration, retry bool, fn func() error) error {
	for {
		err := fn()
		if retry && grpcErrorCode(err) == codes.Unavailable {
			log.Warnf("%s (retrying in %s...)", formatGrpcError(err), interval)
			time.Sleep(interval)
			continue
		}
		return err
	}
}
