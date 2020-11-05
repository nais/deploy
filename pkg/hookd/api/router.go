package api

import (
	"net/http"
	"time"

	"github.com/navikt/deployment/pkg/grpc/deployserver"

	"github.com/go-chi/chi"
	chi_middleware "github.com/go-chi/chi/middleware"
	gh "github.com/google/go-github/v27/github"
	"github.com/navikt/deployment/pkg/azure/discovery"
	"github.com/navikt/deployment/pkg/azure/graphapi"
	api_v1_apikey "github.com/navikt/deployment/pkg/hookd/api/v1/apikey"
	api_v1_deploy "github.com/navikt/deployment/pkg/hookd/api/v1/deploy"
	api_v1_provision "github.com/navikt/deployment/pkg/hookd/api/v1/provision"
	api_v1_status "github.com/navikt/deployment/pkg/hookd/api/v1/status"
	api_v1_teams "github.com/navikt/deployment/pkg/hookd/api/v1/teams"
	"github.com/navikt/deployment/pkg/hookd/config"
	"github.com/navikt/deployment/pkg/hookd/database"
	"github.com/navikt/deployment/pkg/hookd/logproxy"
	"github.com/navikt/deployment/pkg/hookd/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var (
	requestTimeout = time.Second * 10
)

type Middleware func(http.Handler) http.Handler

type Config struct {
	ApiKeyStore                 database.ApiKeyStore
	BaseURL                     string
	DeployServer                deployserver.DeployServer
	Certificates                map[string]discovery.CertificateList
	Clusters                    []string
	DeploymentStore             database.DeploymentStore
	GithubConfig                config.Github
	InstallationClient          *gh.Client
	MetricsPath                 string
	OAuthKeyValidatorMiddleware Middleware
	ProvisionKey                []byte
	TeamClient                  graphapi.Client
	TeamRepositoryStorage       database.RepositoryTeamStore
}

func New(cfg Config) chi.Router {

	prometheusMiddleware := middleware.PrometheusMiddleware("hookd")

	deploymentHandler := &api_v1_deploy.DeploymentHandler{
		APIKeyStorage:   cfg.ApiKeyStore,
		BaseURL:         cfg.BaseURL,
		DeployServer:    cfg.DeployServer,
		Clusters:        cfg.Clusters,
		DeploymentStore: cfg.DeploymentStore,
	}

	teamsHandler := &api_v1_teams.TeamsHandler{
		APIKeyStorage: cfg.ApiKeyStore,
	}

	apikeyHandler := &api_v1_apikey.ApiKeyHandler{
		APIKeyStorage: cfg.ApiKeyStore,
	}

	statusHandler := &api_v1_status.StatusHandler{
		APIKeyStorage:   cfg.ApiKeyStore,
		DeploymentStore: cfg.DeploymentStore,
	}

	provisionHandler := &api_v1_provision.Handler{
		APIKeyStorage: cfg.ApiKeyStore,
		TeamClient:    cfg.TeamClient,
		SecretKey:     cfg.ProvisionKey,
	}

	// Pre-populate request metrics
	for _, code := range api_v1_deploy.StatusCodes {
		prometheusMiddleware.Initialize("/api/v1/deploy", http.MethodPost, code)
	}
	for _, code := range api_v1_status.StatusCodes {
		prometheusMiddleware.Initialize("/api/v1/status", http.MethodPost, code)
	}
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
		if cfg.OAuthKeyValidatorMiddleware != nil {
			r.Route("/apikey", func(r chi.Router) {
				r.Use(cfg.OAuthKeyValidatorMiddleware)
				r.Get("/", apikeyHandler.GetApiKeys)              // -> apikey til alle teams brukeren er autorisert for å se
				r.Get("/{team}", apikeyHandler.GetTeamApiKey)     // -> apikey til dette spesifikke teamet
				r.Post("/{team}", apikeyHandler.RotateTeamApiKey) // -> rotate key (Validere at brukeren er owner av gruppa som eier keyen)
			})
			r.Route("/teams", func(r chi.Router) {
				r.Use(cfg.OAuthKeyValidatorMiddleware)
				r.Get("/", teamsHandler.ServeHTTP) // -> ID og navn (Liste over teams brukeren har tilgang til)
			})
		} else {
			log.Error("Refusing to set up team API key retrieval without OAuth middleware; try configuring --azure-*")
			log.Error("Note: /api/v1/apikey will be unavailable")
			log.Error("Note: /api/v1/teams will be unavailable")
		}
		r.Post("/deploy", deploymentHandler.ServeHTTP)
		r.Post("/status", statusHandler.ServeHTTP)
		if len(cfg.ProvisionKey) == 0 {
			log.Error("Refusing to set up team API provisioning endpoint without pre-shared secret; try using --provision-key")
			log.Error("Note: /api/v1/provision will be unavailable")
		} else {
			r.Post("/provision", provisionHandler.ServeHTTP)
		}
	})

	return router
}
