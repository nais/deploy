package api

import (
	"net/http"
	"time"

	"github.com/nais/deploy/pkg/grpc/dispatchserver"

	"github.com/go-chi/chi"
	chi_middleware "github.com/go-chi/chi/middleware"
	gh "github.com/google/go-github/v41/github"
	api_v1_apikey "github.com/nais/deploy/pkg/hookd/api/v1/apikey"
	api_v1_provision "github.com/nais/deploy/pkg/hookd/api/v1/provision"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/nais/deploy/pkg/hookd/logproxy"
	"github.com/nais/deploy/pkg/hookd/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var requestTimeout = time.Second * 10

type Middleware func(http.Handler) http.Handler

type Config struct {
	ApiKeyStore           database.ApiKeyStore
	BaseURL               string
	DispatchServer        dispatchserver.DispatchServer
	InstallationClient    *gh.Client
	MetricsPath           string
	PSKValidator          func(http.Handler) http.Handler
	ProvisionKey          []byte
	TeamRepositoryStorage database.RepositoryTeamStore
	Projects              map[string]string
	LogLinkFormatter      logproxy.LogLinkFormatter
}

func New(cfg Config) chi.Router {
	prometheusMiddleware := middleware.PrometheusMiddleware("hookd")

	apiKeyHandler := &api_v1_apikey.DefaultApiKeyHandler{
		APIKeyStorage: cfg.ApiKeyStore,
	}

	provisionHandler := &api_v1_provision.Handler{
		APIKeyStorage: cfg.ApiKeyStore,
		SecretKey:     cfg.ProvisionKey,
	}

	goneHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
	}

	// Pre-populate request metrics
	for _, code := range api_v1_provision.StatusCodes {
		prometheusMiddleware.Initialize("/internal/api/v1/provision", http.MethodPost, code)
	}

	// Base settings for all requests
	router := chi.NewRouter()
	router.Use(
		middleware.RequestLogger(),
		prometheusMiddleware.Handler(),
		chi_middleware.StripSlashes,
	)

	router.HandleFunc("/events", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
	})

	// Mount /metrics endpoint with no authentication
	router.Get(cfg.MetricsPath, promhttp.Handler().ServeHTTP)

	// Deployment logs accessible via shorthand URL
	router.HandleFunc("/logs", logproxy.MakeHandler(logproxy.Config{Projects: cfg.Projects, LogLinkFormatter: cfg.LogLinkFormatter}))

	// Public HTTP api/v1 deprecated. Everything from the outside uses gRPC.
	router.HandleFunc("/api/v1", goneHandler)

	router.Route("/internal/api/v1", func(r chi.Router) {
		r.Use(
			chi_middleware.AllowContentType("application/json"),
			chi_middleware.Timeout(requestTimeout),
		)

		if len(cfg.ProvisionKey) == 0 {
			log.Error("Refusing to set up internal team API provisioning endpoint without pre-shared secret; try using --provision-key")
			log.Error("Note: /internal/api/v1/provision will be unavailable")
		} else {
			r.Post("/provision", provisionHandler.Provision)
			r.Post("/apikey", provisionHandler.ApiKey)
		}

		if cfg.PSKValidator == nil {
			log.Error("Refusing to set up internal console API endpoint without psk validator; try configuring --frontend-keys")
			log.Error("Note: /internal/api/v1/console will be unavailable")
		} else {
			r.Route("/console", func(r chi.Router) {
				r.Use(cfg.PSKValidator)
				r.Get("/apikey/{team}", apiKeyHandler.GetTeamApiKey)
				r.Post("/apikey/{team}", apiKeyHandler.RotateTeamApiKey)
			})
		}
	})

	return router
}
