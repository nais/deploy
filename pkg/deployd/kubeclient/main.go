package kubeclient

import (
	"fmt"
	"os"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	_ "k8s.io/client-go/plugin/pkg/client/auth" // Needed for azure auth side effect

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	ServiceUserTemplate = "serviceuser-%s"
	ClusterName         = "kubernetes"
)

type Client struct {
	Base   kubernetes.Interface
	Config *rest.Config
}

type TeamClientProvider interface {
	TeamClient(team string) (TeamClient, error)
}

func New() (*Client, error) {
	config, err := defaultConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		Base:   client,
		Config: config,
	}, nil
}

func (c *Client) teamConfig(team string) (*clientcmdapi.Config, error) {
	serviceAccountName := serviceAccountName(team)

	// Get service account for this team.
	serviceAccount, err := serviceAccount(c.Base, serviceAccountName, team)
	if err != nil {
		return nil, fmt.Errorf("while retrieving service account: %s", err)
	}

	// get service account secret token
	secret, err := serviceAccountSecret(c.Base, *serviceAccount)
	if err != nil {
		return nil, fmt.Errorf("while retrieving secret token: %s", err)
	}

	authInfo := authInfo(*secret)

	teamConfig := clientcmdapi.NewConfig()
	teamConfig.AuthInfos[serviceAccountName] = &authInfo
	teamConfig.Clusters[ClusterName] = &clientcmdapi.Cluster{
		Server:                   c.Config.Host,
		InsecureSkipTLSVerify:    c.Config.Insecure,
		CertificateAuthority:     c.Config.CAFile,
		CertificateAuthorityData: c.Config.CAData,
	}
	teamConfig.Contexts[ClusterName] = &clientcmdapi.Context{
		Namespace: team,
		AuthInfo:  serviceAccountName,
		Cluster:   ClusterName,
	}
	teamConfig.CurrentContext = ClusterName

	return teamConfig, nil
}

// TeamClient returns a Kubernetes REST client tailored for a specific team.
// The user is the `serviceuser-TEAM` in the team's self-named namespace.
func (c *Client) TeamClient(team string) (TeamClient, error) {
	config, err := c.teamConfig(team)
	if err != nil {
		return nil, err
	}

	output, err := clientcmd.Write(*config)
	if err != nil {
		return nil, fmt.Errorf("generating team Kubeconfig: %s", err)
	}

	rc, err := clientcmd.RESTConfigFromKubeConfig(output)
	if err != nil {
		return nil, fmt.Errorf("generating Kubernetes REST client config: %s", err)
	}

	k, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return nil, fmt.Errorf("unable to generate Kubernetes client: %s", err)
	}

	d, err := dynamic.NewForConfig(rc)
	if err != nil {
		return nil, fmt.Errorf("unable to generate dynamic client: %s", err)
	}

	return &teamClient{
		structuredClient:   k,
		unstructuredClient: d,
	}, nil
}

func defaultConfig() (*rest.Config, error) {
	cfg, err := rest.InClusterConfig()
	if err == nil {
		log.Tracef("Running inside Kubernetes, using in-cluster configuration")
		return cfg, nil
	}
	cf := kubeconfig()
	log.Tracef("Not running inside Kubernetes, using configuration file %s", cf)
	return clientcmd.BuildConfigFromFlags("", cf)
}

func kubeconfig() string {
	env, found := os.LookupEnv("KUBECONFIG")
	if !found {
		return clientcmd.RecommendedHomeFile
	}
	return env
}

func serviceAccountName(team string) string {
	return fmt.Sprintf(ServiceUserTemplate, team)
}

func serviceAccount(client kubernetes.Interface, serviceAccountName, namespace string) (*v1.ServiceAccount, error) {
	log.Tracef("Attempting to retrieve service account '%s' in namespace %s", serviceAccountName, namespace)
	return client.CoreV1().ServiceAccounts(namespace).Get(serviceAccountName, metav1.GetOptions{})
}

func serviceAccountSecret(client kubernetes.Interface, serviceAccount v1.ServiceAccount) (*v1.Secret, error) {
	if len(serviceAccount.Secrets) == 0 {
		return nil, fmt.Errorf("no secret associated with service account '%s'", serviceAccount.Name)
	}
	secretRef := serviceAccount.Secrets[0]
	log.Tracef("Attempting to retrieve secret '%s' in namespace %s", secretRef.Name, serviceAccount.Namespace)
	return client.CoreV1().Secrets(serviceAccount.Namespace).Get(secretRef.Name, metav1.GetOptions{})
}

func createServiceAccount(client kubernetes.Interface, serviceAccountName, namespace string) (*v1.ServiceAccount, error) {
	log.Tracef("Attempting to create service account '%s' in namespace %s", serviceAccountName, namespace)
	serviceAccount := v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}
	return client.CoreV1().ServiceAccounts(namespace).Create(&serviceAccount)
}

func authInfo(secret v1.Secret) clientcmdapi.AuthInfo {
	return clientcmdapi.AuthInfo{
		Token: string(secret.Data["token"]),
	}
}
