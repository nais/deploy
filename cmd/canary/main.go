// Command canary runs the nais-deploy canary in the v3 world.
//
// Usage:
//
//	canary provision  # ensure team + service account exist (run on Helm apply)
//	canary deploy     # mint a short-lived token and deploy the canary
package main

import (
	"context"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/nais/deploy/internal/canary"
)

func main() {
	log.SetFormatter(&log.TextFormatter{DisableTimestamp: true})

	if len(os.Args) < 2 {
		log.Error("usage: canary <provision|deploy>")
		os.Exit(1)
	}

	cfg, err := canary.ConfigFromEnv()
	if err != nil {
		log.Errorf("%s", err)
		os.Exit(1)
	}

	ctx := context.Background()
	switch os.Args[1] {
	case "provision":
		err = canary.Provision(ctx, cfg)
	case "deploy":
		err = canary.Deploy(ctx, cfg)
	default:
		log.Errorf("unknown subcommand %q (want provision|deploy)", os.Args[1])
		os.Exit(1)
	}

	if err != nil {
		log.Errorf("%s", err)
		os.Exit(1)
	}
}
