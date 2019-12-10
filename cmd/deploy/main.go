package main

import (
	"net/http"
	"os"

	"github.com/navikt/deployment/deploy/pkg/deployer"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

func main() {
	cfg := deployer.NewConfig()
	d := deployer.Deployer{Client: http.DefaultClient, DeployServer: cfg.DeployServerURL}
	code, err := d.Run(cfg)

	if err != nil {
		if code == deployer.ExitInvocationFailure {
			flag.Usage()
		}

		log.Errorf("fatal: %s", err)
	}

	os.Exit(int(code))
}
