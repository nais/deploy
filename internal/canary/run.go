package canary

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func gauge(name, help string) prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      name,
		Help:      help,
		Namespace: "nais",
		Subsystem: "deploy_canary",
	})
}

var (
	TimeSinceDeploy = gauge("time_since_deploy", "Seconds since the latest deploy of this application")
	StartTimestamp  = gauge("start_timestamp", "Start time of the application")
	DeployTimestamp = gauge("deploy_timestamp", "Timestamp when the deploy of this application was triggered in the pipeline")
)

func Run(deployTime int64) error {
	if v, ok := os.LookupEnv("DEPLOY_START"); ok {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			deployTime = parsed
		}
	}

	prometheus.MustRegister(TimeSinceDeploy)
	prometheus.MustRegister(StartTimestamp)
	prometheus.MustRegister(DeployTimestamp)

	StartTimestamp.SetToCurrentTime()
	DeployTimestamp.Set(float64(deployTime) / 1e9)
	TimeSinceDeploy.Set(timeSinceDeploy(deployTime))

	tick := time.NewTicker(time.Second)
	go func() {
		for range tick.C {
			TimeSinceDeploy.Set(timeSinceDeploy(deployTime))
		}
	}()

	mux := http.DefaultServeMux
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "🐣 pip pip")
	})

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("running @ :8080")

	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("failed to listen and serve: %s", err)
	}

	return nil
}

func timeSinceDeploy(deployTime int64) float64 {
	deployStartTime := time.Unix(0, deployTime)
	return time.Since(deployStartTime).Seconds()
}
