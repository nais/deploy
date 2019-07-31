package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"github.com/navikt/deployment/common/pkg/logging"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	"github.com/navikt/deployment/pkg/token-generator/server"
	"github.com/navikt/deployment/pkg/token-generator/sinks/circleci"
	"github.com/navikt/deployment/pkg/token-generator/sources/github"
	"github.com/navikt/deployment/pkg/token-generator/types"
	log "github.com/sirupsen/logrus"
)

// Configure all credential sources and return them.
func configureSources(cfg Config) (*types.SourceFuncs, error) {
	keyBytes, err := ioutil.ReadFile(cfg.Github.Keyfile)
	if err != nil {
		return nil, fmt.Errorf("read private key: %s", err)
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %s", err)
	}

	// Check that creation of a single token succeeds. If it doesn't, there is
	// a high chance that we can't sign any tokens at all.
	_, err = github_source.AppToken(key, cfg.Github.AppID, time.Second)
	if err != nil {
		return nil, fmt.Errorf("test token generation: %s", err)
	}

	return &types.SourceFuncs{
		"github": func(request types.Request) (*types.Credentials, error) {
			return github_source.Credentials(github_source.InstallationTokenRequest{
				InstallationID: cfg.Github.InstallationID,
				ApplicationID:  cfg.Github.AppID,
				Key:            key,
			})
		},
	}, nil

}

// Configure all credential sinks and return them.
func configureSinks(cfg Config) (*types.SinkFuncs, error) {
	return &types.SinkFuncs{
		"circleci": func(request types.Request, credentials types.Credentials) error {
			return circleci_sink.Sink(request, credentials, cfg.CircleCI.Apitoken)
		},
	}, nil
}

func issuer(sources types.SourceFuncs, sinks types.SinkFuncs) server.Issuer {
	return func(request types.Request) error {
		var credentials = make([]types.Credentials, 0)
		var logger = log.WithFields(log.Fields{
			"requestID": uuid.New().String(),
		})

		// Draw credentials from all sources
		for name, source := range sources {
			for _, requestedSource := range request.Sources.Values() {
				if name == types.Source(requestedSource) {
					credential, err := source(request)
					if err != nil {
						logger.Errorf("sources: %s: %s", name, err)
						return fmt.Errorf("unable to get credentials from %s", name)
					}
					credential.Source = name
					credentials = append(credentials, *credential)
					log.Tracef("sources: %s: got credentials", name)
				}
			}
		}

		// Push credentials to all sinks
		for name, sink := range sinks {
			for _, requestedSink := range request.Sinks.Values() {
				if name == types.Sink(requestedSink) {
					for _, credential := range credentials {
						err := sink(request, credential)
						if err != nil {
							logger.Errorf("sinks: %s: %s", name, err)
							return fmt.Errorf("unable to push credentials to %s", name)
						}
						log.Tracef("sinks: %s: pushed credentials", name)
					}
				}
			}
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

	sources, err := configureSources(*cfg)
	if err != nil {
		return err
	}

	sinks, err := configureSinks(*cfg)
	if err != nil {
		return err
	}

	handler := server.New(issuer(*sources, *sinks))

	return http.ListenAndServe(cfg.Bind, handler)
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
	}
}
