// Command deploy-action is the Go implementation of the "Nais Deploy v3"
// GitHub Action logic previously implemented in actions/deploy/entrypoint.sh.
// It reads the same environment variables (CLUSTER, RESOURCE, TEAM, VARS,
// VAR, IMAGE, WORKLOAD_IMAGE, WAIT, TIMEOUT, DRY_RUN), templates the given
// resource files, auto-detects the team if not set, and applies the result
// with `nais alpha apply`.
package main

import (
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/nais/deploy/pkg/deployaction"
)

func main() {
	log.SetFormatter(&log.TextFormatter{DisableTimestamp: true})

	cfg, err := deployaction.ConfigFromEnv()
	if err != nil {
		log.Errorf("::error::%s", err)
		os.Exit(1)
	}

	if err := deployaction.Run(cfg); err != nil {
		os.Exit(1)
	}
}
