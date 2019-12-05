package main

import (
	"net/http"
	"os"

	"github.com/navikt/deployment/deploy/pkg/deployer"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

const defaultDeployServer = "https://deployment.prod-sbs.nais.io"

func main() {
	d := deployer.Deployer{Client: http.DefaultClient, DeployServer: defaultDeployServer}
	code, err := d.Run(deployer.NewConfig())

	if err != nil {
		if code == deployer.ExitInvocationFailure {
			flag.Usage()
		}

		log.Errorf("fatal: %s", err)
	}

	os.Exit(int(code))
}
