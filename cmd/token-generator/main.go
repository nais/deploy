package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/navikt/deployment/common/pkg/logging"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	"github.com/navikt/deployment/pkg/token-generator/server"
	"github.com/navikt/deployment/pkg/token-generator/sinks/circleci"
	"github.com/navikt/deployment/pkg/token-generator/sources/github"
	"github.com/navikt/deployment/pkg/token-generator/types"
	log "github.com/sirupsen/logrus"
)

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
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

	router := chi.NewRouter()
	router.Use(
		middleware.AllowContentType("application/json"),
		middleware.Timeout(cfg.Http.Timeout),
	)
	router.Post("/create", handler.ServeHTTP)

	return http.ListenAndServe(cfg.Bind, router)
}

// Configure all credential sources and return them.
func configureSources(cfg Config) (*types.SourceFuncs, error) {
	githubKey, err := github_source.RSAPrivateKeyFromPEMFile(cfg.Github.Keyfile)
	if err != nil {
		return nil, err
	}

	return &types.SourceFuncs{
		"github": func(request types.Request) (*types.Credentials, error) {
			return github_source.Credentials(github_source.InstallationTokenRequest{
				Context:        request.Context,
				InstallationID: cfg.Github.InstallationID,
				ApplicationID:  cfg.Github.AppID,
				Key:            githubKey,
			})
		},
	}, nil

}

// Configure all credential sinks and return them.
func configureSinks(cfg Config) (*types.SinkFuncs, error) {
	return &types.SinkFuncs{
		"circleci": func(request types.Request, credentials types.Credentials) error {
			return circleci_sink.Sink(request, credentials, cfg.CircleCI.Apitoken, http.DefaultClient)
		},
	}, nil
}

// issuer return a closure that will be called by incoming token generation requests.
// The closure finds the intersect between configured and requested sinks and sources,
// then retrieves credentials from those sources, and stores the credentials to those sinks.
//
// The interface between a sink and a source is the Credentials object. Both sinks and sources
// will receive the Request object with the original request data.
//
// If any request fails, the function aborts and returns an error. Operations are not atomic.
//
// The caller will receive a generic error message while the true error is logged.
func issuer(sources types.SourceFuncs, sinks types.SinkFuncs) server.Issuer {
	return func(request types.Request) error {
		var credentials = make([]types.Credentials, 0)
		var logger = log.WithFields(log.Fields{
			"correlationID": request.ID,
			"repository":    request.Repository,
			"sources":       request.Sources.Values(),
			"sinks":         request.Sinks.Values(),
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
					logger.Tracef("sources: %s: got credentials", name)
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
						logger.Tracef("sinks: %s: pushed credentials", name)
					}
				}
			}
		}

		return nil
	}
}
