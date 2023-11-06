package api

import (
	"net/http"
	"time"

	"github.com/nais/deploy/pkg/grpc/dispatchserver"

	"github.com/go-chi/chi"
	chi_middleware "github.com/go-chi/chi/middleware"
	gh "github.com/google/go-github/v41/github"
	api_v1_apikey "github.com/nais/deploy/pkg/hookd/api/v1/apikey"
	api_v1_dashboard "github.com/nais/deploy/pkg/hookd/api/v1/dashboard"
	api_v1_provision "github.com/nais/deploy/pkg/hookd/api/v1/provision"
	api_v1_teams "github.com/nais/deploy/pkg/hookd/api/v1/teams"
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
	DeploymentStore       database.DeploymentStore
	InstallationClient    *gh.Client
	MetricsPath           string
	ValidatorMiddlewares  chi.Middlewares
	PSKValidator          func(http.Handler) http.Handler
	ProvisionKey          []byte
	TeamRepositoryStorage database.RepositoryTeamStore
	Projects              map[string]string
	LogLinkFormatter      logproxy.LogLinkFormatter
}

func New(cfg Config) chi.Router {
	prometheusMiddleware := middleware.PrometheusMiddleware("hookd")

	teamsHandler := &api_v1_teams.TeamsHandler{
		APIKeyStorage: cfg.ApiKeyStore,
	}

	googleAPIKeyHandler := &api_v1_apikey.GoogleApiKeyHandler{
		APIKeyStorage: cfg.ApiKeyStore,
	}

	apiKeyHandler := &api_v1_apikey.DefaultApiKeyHandler{
		APIKeyStorage: cfg.ApiKeyStore,
	}

	provisionHandler := &api_v1_provision.Handler{
		APIKeyStorage: cfg.ApiKeyStore,
		SecretKey:     cfg.ProvisionKey,
	}

	dashboardHandler := &api_v1_dashboard.Handler{
		DeploymentStore: cfg.DeploymentStore,
	}

	goneHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	}

	// Pre-populate request metrics
	prometheusMiddleware.Initialize("/api/v1/deploy", http.MethodPost, http.StatusGone)
	prometheusMiddleware.Initialize("/api/v1/status", http.MethodPost, http.StatusGone)
	for _, code := range api_v1_provision.StatusCodes {
		prometheusMiddleware.Initialize("/api/v1/provision", http.MethodPost, code)
	}

	// Base settings for all requests
	router := chi.NewRouter()
	router.Use(
		middleware.RequestLogger(),
		prometheusMiddleware.Handler(),
		chi_middleware.StripSlashes,
	)

	router.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	})

	// Mount /metrics endpoint with no authentication
	router.Get(cfg.MetricsPath, promhttp.Handler().ServeHTTP)

	// Deployment logs accessible via shorthand URL
	router.HandleFunc("/logs", logproxy.MakeHandler(logproxy.Config{Projects: cfg.Projects, LogLinkFormatter: cfg.LogLinkFormatter}))

	// Mount /api/v1 for API requests
	// Only application/json content type allowed
	router.Route("/api/v1", func(r chi.Router) {
		r.Use(
			chi_middleware.AllowContentType("application/json"),
			chi_middleware.Timeout(requestTimeout),
		)
		if cfg.ValidatorMiddlewares != nil {
			r.Route("/apikey", func(r chi.Router) {
				r.Use(cfg.ValidatorMiddlewares...)
				r.Get("/", googleAPIKeyHandler.GetApiKeys)              // -> apikey til alle teams brukeren er autorisert for å se
				r.Get("/{team}", googleAPIKeyHandler.GetTeamApiKey)     // -> apikey til dette spesifikke teamet
				r.Post("/{team}", googleAPIKeyHandler.RotateTeamApiKey) // -> rotate key (Validere at brukeren er owner av gruppa som eier keyen)
			})
			r.Route("/teams", func(r chi.Router) {
				r.Use(cfg.ValidatorMiddlewares...)
				r.Get("/", teamsHandler.ServeHTTP) // -> ID og navn (Liste over teams brukeren har tilgang til)
			})
		} else {
			log.Error("Refusing to set up team API key retrieval without validating middlewares; try configuring --frontend-keys and --google-client-id")
			log.Error("Note: /api/v1/apikey will be unavailable")
			log.Error("Note: /api/v1/teams will be unavailable")
		}
		r.Post("/deploy", goneHandler)
		r.Post("/status", goneHandler)
		if len(cfg.ProvisionKey) == 0 {
			log.Error("Refusing to set up team API provisioning endpoint without pre-shared secret; try using --provision-key")
			log.Error("Note: /api/v1/provision will be unavailable")
		} else {
			r.Post("/provision", provisionHandler.ProvisionExternal)
		}
	})

	router.Route("/internal/api/v1", func(r chi.Router) {
		if len(cfg.ProvisionKey) == 0 {
			log.Error("Refusing to set up internal team API provisioning endpoint without pre-shared secret; try using --provision-key")
			log.Error("Note: /internal/api/v1/provision will be unavailable")
		} else {
			r.Post("/provision", provisionHandler.ProvisionInternal)
			r.Post("/apikey", provisionHandler.ApiKey)
			r.Route("/console", func(r chi.Router) {
				r.Use(cfg.PSKValidator)
				r.Get("/apikey/{team}", apiKeyHandler.GetTeamApiKey)
				r.Post("/apikey/{team}", apiKeyHandler.RotateTeamApiKey)
				r.Get("/deployments", dashboardHandler.Deployments)
			})
		}
	})

	return router
}
