package teamconfig

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	ServiceUserTemplate = "serviceuser-%s"
	ClusterName         = "kubernetes"
)

// Generate returns a Kubernetes REST config tailored for a specific team.
// The user is the `serviceuser-TEAM` in the team's self-named namespace.
func Generate(client kubernetes.Interface, config *rest.Config, team string) (*rest.Config, error) {

	// FIXME: implement context all the way up?
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serviceAccountName := serviceAccountName(team)

	// Get service account for this team.
	serviceAccount, err := serviceAccount(ctx, client, serviceAccountName, team)
	if err != nil {
		return nil, fmt.Errorf("while retrieving service account: %s", err)
	}

	// get service account secret token
	secret, err := serviceAccountSecret(ctx, client, *serviceAccount)
	if err != nil {
		return nil, fmt.Errorf("while retrieving secret token: %s", err)
	}

	authInfo := authInfo(*secret)

	apiConfig := clientcmdapi.NewConfig()
	apiConfig.AuthInfos[serviceAccountName] = &authInfo
	apiConfig.Clusters[ClusterName] = &clientcmdapi.Cluster{
		Server:                   config.Host,
		InsecureSkipTLSVerify:    config.Insecure,
		CertificateAuthority:     config.CAFile,
		CertificateAuthorityData: config.CAData,
	}
	apiConfig.Contexts[ClusterName] = &clientcmdapi.Context{
		Namespace: team,
		AuthInfo:  serviceAccountName,
		Cluster:   ClusterName,
	}
	apiConfig.CurrentContext = ClusterName

	output, err := clientcmd.Write(*apiConfig)
	if err != nil {
		return nil, fmt.Errorf("generating team Kubeconfig: %s", err)
	}

	rc, err := clientcmd.RESTConfigFromKubeConfig(output)
	if err != nil {
		return nil, fmt.Errorf("generating Kubernetes REST client config: %s", err)
	}

	return rc, err
}

func serviceAccountName(team string) string {
	return fmt.Sprintf(ServiceUserTemplate, team)
}

func serviceAccount(ctx context.Context, client kubernetes.Interface, serviceAccountName, namespace string) (*v1.ServiceAccount, error) {
	log.Tracef("Attempting to retrieve service account '%s' in namespace %s", serviceAccountName, namespace)
	return client.CoreV1().ServiceAccounts(namespace).Get(ctx, serviceAccountName, metav1.GetOptions{})
}

func serviceAccountSecret(ctx context.Context, client kubernetes.Interface, serviceAccount v1.ServiceAccount) (*v1.Secret, error) {
	if len(serviceAccount.Secrets) == 0 {
		return nil, fmt.Errorf("no secret associated with service account '%s'", serviceAccount.Name)
	}
	secretRef := serviceAccount.Secrets[0]
	log.Tracef("Attempting to retrieve secret '%s' in namespace %s", secretRef.Name, serviceAccount.Namespace)
	return client.CoreV1().Secrets(serviceAccount.Namespace).Get(ctx, secretRef.Name, metav1.GetOptions{})
}

func authInfo(secret v1.Secret) clientcmdapi.AuthInfo {
	return clientcmdapi.AuthInfo{
		Token: string(secret.Data["token"]),
	}
}
