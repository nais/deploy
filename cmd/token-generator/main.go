package main

import (
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/navikt/deployment/common/pkg/logging"
	"github.com/navikt/deployment/hookd/pkg/github"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	"github.com/navikt/deployment/pkg/circleci/pusher"
	"github.com/navikt/deployment/pkg/github/tokens"
	"github.com/navikt/deployment/pkg/token-generator/server"
	log "github.com/sirupsen/logrus"
)

func issueToCircleCI(request server.Request, envVar pusher.EnvVar, apiToken string) error {
	org, repo, err := github.SplitFullname(request.CircleCI.Repository)
	if err != nil {
		return err
	}

	if err = pusher.SetEnvironmentVariable(envVar, org, repo, apiToken); err != nil {
		return fmt.Errorf("CircleCI: %s", err)
	}

	log.Infof("Issued GitHub token to CircleCI repository %s", request.CircleCI.Repository)

	return nil
}

func issuer(key *rsa.PrivateKey, cfg Config) server.Issuer {
	return func(request server.Request) error {
		appToken, err := tokens.AppToken(key, cfg.Github.AppID, cfg.Github.Validity)
		if err != nil {
			return fmt.Errorf("generate app token: %s", err)
		}

		installationToken, err := tokens.InstallationToken(appToken, cfg.Github.InstallationID)
		if err != nil {
			return fmt.Errorf("generate installation token: %s", err)
		}

		if len(request.CircleCI.Repository) > 0 {
			env := pusher.EnvVar{
				Name:  cfg.Github.EnvVarName,
				Value: installationToken,
			}

			return issueToCircleCI(request, env, cfg.CircleCI.Apitoken)
		}

		return nil
	}
}

func run() error {
	var err error
	var cfg *Config

	cfg, err = configuration()
	if err != nil {
		return err
	}

	if err = logging.Setup(cfg.Log.Level, cfg.Log.Format); err != nil {
		return err
	}

	printConfig(redactKeys)

	_, err = persistence.NewS3StorageBackend(cfg.S3)
	if err != nil {
		return fmt.Errorf("while setting up S3 backend: %s", err)
	}

	keyBytes, err := ioutil.ReadFile(cfg.Github.Keyfile)
	if err != nil {
		return fmt.Errorf("read private key: %s", err)
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
	if err != nil {
		return fmt.Errorf("parse private key: %s", err)
	}

	// Check that creation of a single token succeeds. If it doesn't, there is
	// a high chance that we can't sign any tokens at all.
	_, err = tokens.AppToken(key, cfg.Github.AppID, time.Second)
	if err != nil {
		return fmt.Errorf("test token generation: %s", err)
	}

	handler := server.New(issuer(key, *cfg))

	return http.ListenAndServe(cfg.Bind, handler)
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
	}
}
