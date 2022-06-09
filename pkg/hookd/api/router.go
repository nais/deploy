package api

import (
	"net/http"
	"time"

	"github.com/nais/deploy/pkg/grpc/dispatchserver"

	"github.com/go-chi/chi"
	chi_middleware "github.com/go-chi/chi/middleware"
	gh "github.com/google/go-github/v41/github"
	"github.com/nais/deploy/pkg/azure/discovery"
	"github.com/nais/deploy/pkg/azure/graphapi"
	api_v1_apikey "github.com/nais/deploy/pkg/hookd/api/v1/apikey"
	api_v1_dashboard "github.com/nais/deploy/pkg/hookd/api/v1/dashboard"
	api_v1_provision "github.com/nais/deploy/pkg/hookd/api/v1/provision"
	api_v1_teams "github.com/nais/deploy/pkg/hookd/api/v1/teams"
	"github.com/nais/deploy/pkg/hookd/config"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/nais/deploy/pkg/hookd/logproxy"
	"github.com/nais/deploy/pkg/hookd/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var requestTimeout = time.Second * 10

type GroupProvider int

const (
	GroupProviderGoogle GroupProvider = iota
	GroupProviderAzure
)

type Middleware func(http.Handler) http.Handler

type Config struct {
	ApiKeyStore           database.ApiKeyStore
	BaseURL               string
	DispatchServer        dispatchserver.DispatchServer
	Certificates          map[string]discovery.CertificateList
	DeploymentStore       database.DeploymentStore
	GithubConfig          config.Github
	InstallationClient    *gh.Client
	MetricsPath           string
	ValidatorMiddlewares  chi.Middlewares
	ProvisionKey          []byte
	TeamClient            graphapi.Client
	TeamRepositoryStorage database.RepositoryTeamStore
	GroupProvider         GroupProvider
}

func New(cfg Config) chi.Router {
	prometheusMiddleware := middleware.PrometheusMiddleware("hookd")

	teamsHandler := &api_v1_teams.TeamsHandler{
		APIKeyStorage: cfg.ApiKeyStore,
	}

	var apikeyHandler api_v1_apikey.ApiKeyHandler
	if cfg.GroupProvider == GroupProviderAzure {
		apikeyHandler = &api_v1_apikey.AzureApiKeyHandler{
			APIKeyStorage: cfg.ApiKeyStore,
		}
	} else {
		apikeyHandler = &api_v1_apikey.GoogleApiKeyHandler{
			APIKeyStorage: cfg.ApiKeyStore,
		}
	}

	provisionHandler := &api_v1_provision.Handler{
		APIKeyStorage: cfg.ApiKeyStore,
		TeamClient:    cfg.TeamClient,
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
	router.HandleFunc("/logs", logproxy.HandleFunc)

	// Mount /api/v1 for API requests
	// Only application/json content type allowed
	router.Route("/api/v1", func(r chi.Router) {
		r.Use(
			chi_middleware.AllowContentType("application/json"),
			chi_middleware.Timeout(requestTimeout),
		)
		r.Route("/dashboard", func(r chi.Router) {
			if cfg.ValidatorMiddlewares != nil {
				r.Use(cfg.ValidatorMiddlewares...)
			}
			r.Get("/deployments", dashboardHandler.Deployments)
			r.Get("/deployments/{id}", dashboardHandler.Deployments)
		})
		if cfg.ValidatorMiddlewares != nil {
			r.Route("/apikey", func(r chi.Router) {
				r.Use(cfg.ValidatorMiddlewares...)
				r.Get("/", apikeyHandler.GetApiKeys)              // -> apikey til alle teams brukeren er autorisert for å se
				r.Get("/{team}", apikeyHandler.GetTeamApiKey)     // -> apikey til dette spesifikke teamet
				r.Post("/{team}", apikeyHandler.RotateTeamApiKey) // -> rotate key (Validere at brukeren er owner av gruppa som eier keyen)
			})
			r.Route("/teams", func(r chi.Router) {
				r.Use(cfg.ValidatorMiddlewares...)
				r.Get("/", teamsHandler.ServeHTTP) // -> ID og navn (Liste over teams brukeren har tilgang til)
			})
		} else {
			log.Error("Refusing to set up team API key retrieval without validating middlewares; try configuring --azure-* or --frontend-keys and --google-client-id")
			log.Error("Note: /api/v1/apikey will be unavailable")
			log.Error("Note: /api/v1/teams will be unavailable")
		}
		r.Post("/deploy", goneHandler)
		r.Post("/status", goneHandler)
		if len(cfg.ProvisionKey) == 0 {
			log.Error("Refusing to set up team API provisioning endpoint without pre-shared secret; try using --provision-key")
			log.Error("Note: /api/v1/provision will be unavailable")
		} else {
			r.Post("/provision", provisionHandler.ServeHTTP)
		}
	})

	return router
}
