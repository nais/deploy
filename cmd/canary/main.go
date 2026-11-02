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
	"time"

	"log/slog"

	"github.com/nais/deploy/internal/canary"
)

func main() {
	deployTime := time.Now().UnixNano()

	if len(os.Args) < 2 {
		slog.Error("usage: canary <provision|deploy|run")
		os.Exit(1)
	}

	cfg, err := canary.ConfigFromEnv()
	if err != nil {
		slog.Error("failed parsing config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	switch os.Args[1] {
	case "provision":
		err = canary.Provision(ctx, cfg)
	case "deploy":
		err = canary.Deploy(ctx, cfg)
	case "run":
		err = canary.Run(deployTime)
	default:
		slog.Error("unknown subcommand, supported commands provision|deploy|run", "command", os.Args[1])
		os.Exit(1)
	}

	if err != nil {
		slog.Error("command failed", "error", err, "command", os.Args[1])
		os.Exit(1)
	}
}
