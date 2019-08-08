package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/go-chi/chi"
	chi_middleware "github.com/go-chi/chi/middleware"
	"github.com/navikt/deployment/common/pkg/logging"
	"github.com/navikt/deployment/pkg/token-generator/apikeys"
	"github.com/navikt/deployment/pkg/token-generator/azure"
	"github.com/navikt/deployment/pkg/token-generator/metrics"
	"github.com/navikt/deployment/pkg/token-generator/middleware"
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
	var azureCertificates map[string]azure.CertificateList
	var apiKeySource apikeys.Source

	cfg, err = configuration()
	if err != nil {
		return err
	}

	if err = logging.Setup(cfg.Log.Level, cfg.Log.Format); err != nil {
		return err
	}

	printConfig(redactKeys)

	log.Infof("Configuring sources")
	sources, err := configureSources(*cfg)
	if err != nil {
		return err
	}

	log.Infof("Configuring sinks")
	sinks, err := configureSinks(*cfg)
	if err != nil {
		return err
	}

	if len(cfg.Storage.Bucketname) == 0 {
		log.Infof("Configuring in-memory API key storage")
		apiKeySource = apikeys.NewMemoryStore()
	} else {
		log.Infof("Configuring Google Cloud Storage for API keys")
		apiKeySource, err = apikeys.NewGoogleCloudStore(cfg.Storage.Keyfile, cfg.Storage.Bucketname)
		if err != nil {
			return err
		}
	}

	log.Infof("(fixme) write api key")
	err = apiKeySource.Write("admin", "admin")
	if err != nil {
		return err
	}
	log.Infof("(fixme) API keys for this service is http basic auth `admin:admin`")

	if cfg.Azure.HasConfig() {
		log.Infof("Discover Microsoft signing certificates from %s", cfg.Azure.DiscoveryURL)
		azureKeyDiscovery, err := azure.DiscoverURL(cfg.Azure.DiscoveryURL)
		if err != nil {
			return err
		}

		log.Infof("Decoding certificates for %d keys", len(azureKeyDiscovery.Keys))
		azureCertificates, err = azureKeyDiscovery.Map()
		if err != nil {
			return err
		}
	}

	oauthUserConfig := azure.NewUserConfig(
		cfg.Azure.ClientID,
		cfg.Azure.ClientSecret,
		cfg.Azure.Tenant,
		cfg.Azure.RedirectURL,
	)

	oauthServerConfig := azure.NewApplicationConfig(
		cfg.Azure.ClientID,
		cfg.Azure.ClientSecret,
		cfg.Azure.Tenant,
	)

	tokenIssuer := server.New(issuer(*sources, *sinks))

	apikeyIssuer := server.NewAPIKeyIssuer(apiKeySource, oauthServerConfig)

	authHandler := server.NewAuthHandler(oauthUserConfig)

	router := chi.NewRouter()

	// Base settings for all requests
	router.Use(
		middleware.PrometheusMiddlewareHandler("token-generator"),
		chi_middleware.Logger,
		chi_middleware.StripSlashes,
		chi_middleware.Timeout(cfg.Http.Timeout),
	)

	// Mount /metrics endpoint with no authentication
	router.Get("/metrics", metrics.Handler().ServeHTTP)

	// Mount /api/v1/tokens for API requests
	// All requests must provide a valid API key.
	router.Route("/api/v1/tokens", func(r chi.Router) {
		r.Use(middleware.ApiKeyMiddlewareHandler(apiKeySource))
		r.Use(chi_middleware.AllowContentType("application/json"))
		r.Post("/", tokenIssuer.ServeHTTP)
	})

	// These endpoints require OAuth configuration.
	if cfg.Azure.HasConfig() {

		// Mount /api/v1/apikeys for authenticated requests.
		// Requests must provide valid JWT tokens,
		// otherwise they will be redirected to /auth/login.
		router.Route("/api/v1/apikeys", func(r chi.Router) {
			r.Use(chi_middleware.AllowContentType("application/json"))
			r.Use(middleware.JWTMiddlewareHandler(azureCertificates))
			r.Post("/", apikeyIssuer.ServeHTTP)
		})

		// OAuth 2.0 auth code flow using Azure.
		router.Route("/auth", func(r chi.Router) {
			r.Get("/login", authHandler.Authorize)
			r.Get("/callback", authHandler.Callback)
			r.Get("/echo", authHandler.Echo)
		})

	} else {
		log.Warnf("Endpoints /auth and /api/v1/apikeys are unavailable due to missing Azure configuration")
	}

	log.Infof("Ready to accept connections")

	return http.ListenAndServe(cfg.Bind, router)
}

// Configure all credential sources and return them.
func configureSources(cfg Config) (*types.SourceFuncs, error) {
	githubKey, err := github_source.RSAPrivateKeyFromPEMFile(cfg.Github.Keyfile)
	if err != nil {
		return nil, err
	}

	return &types.SourceFuncs{
		"github": func(request types.TokenIssuerRequest) (*types.Credentials, error) {
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
		"circleci": func(request types.TokenIssuerRequest, credentials types.Credentials) error {
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
	return func(request types.TokenIssuerRequest) error {
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
