package main

import (
	"os"

	"github.com/navikt/deployment/pkg/deployer"
	"github.com/navikt/deployment/pkg/pb"

	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

func main() {
	cfg := deployer.NewConfig()

	deployer.SetupLogging(cfg)

	grpcConnection, err := deployer.NewGrpcConnection(cfg)
	if err != nil {
		log.Errorf("fatal: %s", err)
		os.Exit(int(deployer.ExitUnavailable))
	}
	defer grpcConnection.Close()

	grpcClient := pb.NewDeployClient(grpcConnection)

	d := deployer.Deployer{Client: grpcClient}

	code, err := d.Run(cfg)

	if err != nil {
		if code == deployer.ExitInvocationFailure {
			flag.Usage()
		}
		log.Errorf("fatal: %s", err)
	}

	os.Exit(int(code))
}
