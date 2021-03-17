package kubeclient

import (
	"os"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Return auto-detected system/user Kubernetes configuration,
// either from in-cluster autoconfiguration or a $KUBECONFIG file.
func SystemConfig() (*rest.Config, error) {
	cfg, err := rest.InClusterConfig()
	if err == nil {
		log.Tracef("Running inside Kubernetes, using in-cluster configuration")
		return cfg, nil
	}
	cf := kubeConfigPath()
	log.Tracef("Not running inside Kubernetes, using configuration file %s", cf)
	return clientcmd.BuildConfigFromFlags("", cf)
}

func DefaultClient() (Interface, error) {
	config, err := SystemConfig()
	if err != nil {
		return nil, err
	}
	return New(config)
}

func kubeConfigPath() string {
	env, found := os.LookupEnv("KUBECONFIG")
	if !found {
		return clientcmd.RecommendedHomeFile
	}
	return env
}
