// There are two entrypoints, both driven by environment variables:
//
//   - provision: idempotently ensure the nais-verification team and the
//     canary service account exist (via the open Nais API gRPC endpoint).
//   - deploy: mint a short-lived service account token and apply the canary
//     Application manifest to each environment via the Nais API REST apply
//     endpoint.
package canary

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	// TeamSlug is the team the canary runs as.
	TeamSlug = "nais-verification"

	// ServiceAccountName is the canary's service account within the Nais team.
	ServiceAccountName = "deploy-canary"

	// RoleName grants k8s_resources:apply (among other team-member rights)
	RoleName = "Team member"

	teamPurpose               = "A place for Nais to run verification workloads"
	teamChannel               = "#nais"
	serviceAccountDescription = "Service account used by the deploy canary"
)

type Config struct {
	GRPCTarget   string
	APIBaseURL   string
	Environments []string
}

func ConfigFromEnv() (*Config, error) {
	cfg := &Config{
		GRPCTarget: os.Getenv("GRPC_TARGET"),
		APIBaseURL: strings.TrimSuffix(os.Getenv("API_BASE_URL"), "/"),
	}

	if envs := os.Getenv("ENVIRONMENTS"); envs != "" {
		for e := range strings.SplitSeq(envs, ",") {
			if e = strings.TrimSpace(e); e != "" {
				cfg.Environments = append(cfg.Environments, e)
			}
		}
	}

	return cfg, nil
}

func (c *Config) validateProvision() error {
	if c.GRPCTarget == "" {
		return fmt.Errorf("GRPC_TARGET is required")
	}
	return nil
}

func (c *Config) validateDeploy() error {
	if c.GRPCTarget == "" {
		return fmt.Errorf("GRPC_TARGET is required")
	}
	if c.APIBaseURL == "" {
		return fmt.Errorf("API_BASE_URL is required")
	}
	if len(c.Environments) == 0 {
		return fmt.Errorf("ENVIRONMENTS is required")
	}
	return nil
}

// tokenExpiry returns tomorrow, used as the day-granular expiry for the
// short-lived token minted per deploy run.
func tokenExpiry(now time.Time) time.Time {
	return now.Add(24 * time.Hour)
}
