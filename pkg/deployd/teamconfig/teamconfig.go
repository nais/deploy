package teamconfig

import (
	"fmt"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
)

const (
	UserNameTemplate    = "system:serviceaccount:%s:%s"
	ServiceUserTemplate = "serviceuser-%s"
)

// Generate returns a Kubernetes REST config tailored for a specific team.
// The user is the `serviceuser-TEAM` in the team's self-named namespace.
func Generate(config rest.Config, team string) (*rest.Config, error) {
	serviceAccountName := serviceAccountName(team)

	config.Impersonate = rest.ImpersonationConfig{
		UserName: impersonationUserName(serviceAccountName, team),
	}
	return &config, nil
}

func impersonationUserName(serviceAccountName, namespace string) string {
	return fmt.Sprintf(UserNameTemplate, namespace, serviceAccountName)
}

func serviceAccountName(team string) string {
	return fmt.Sprintf(ServiceUserTemplate, team)
}
