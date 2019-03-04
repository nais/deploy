package github

import (
	"context"
	"fmt"
	"github.com/bradleyfalzon/ghinstallation"
	gh "github.com/google/go-github/v23/github"
	types "github.com/navikt/deployment/common/pkg/deployment"
	"net/http"
	"strings"
)

func SplitFullname(fullName string) (string, string, error) {
	parts := strings.Split(fullName, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("repository name %s is not in the format OWNER/NAME", fullName)
	}
	return parts[0], parts[1], nil
}

func ApplicationClient(appId int, keyFile string) (*gh.Client, error) {
	itr, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, appId, keyFile)
	if err != nil {
		return nil, err
	}
	return gh.NewClient(&http.Client{Transport: itr}), nil
}

func InstallationClient(appId, installId int, keyFile string) (*gh.Client, error) {
	itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appId, installId, keyFile)
	if err != nil {
		return nil, err
	}
	return gh.NewClient(&http.Client{Transport: itr}), nil
}

func CreateHook(client *gh.Client, r gh.Repository, url string, secret string) (*gh.Hook, error) {
	active := true
	webhook := &gh.Hook{
		Events: []string{
			"deployment",
		},
		Active: &active,
		Config: map[string]interface{}{
			"url":          url,
			"content_type": "json",
			"insecure_ssl": "0",
			"secret":       secret,
		},
	}

	owner, name, err := SplitFullname(r.GetFullName())
	if err != nil {
		return nil, err
	}
	webhook, _, err = client.Repositories.CreateHook(context.Background(), owner, name, webhook)
	if err != nil {
		return nil, err
	}

	return webhook, nil
}

func DeleteHook(client *gh.Client, r gh.Repository, id int64) error {
	owner, name, err := SplitFullname(r.GetFullName())
	if err != nil {
		return err
	}
	_, err = client.Repositories.DeleteHook(context.Background(), owner, name, id)
	return err
}

func CreateDeploymentStatus(client *gh.Client, m *types.DeploymentStatus) (*gh.DeploymentStatus, *gh.Response, error) {
	deployment := m.GetDeployment()
	if deployment == nil {
		return nil, nil, fmt.Errorf("empty deployment")
	}

	repo := deployment.GetRepository()
	if repo == nil {
		return nil, nil, fmt.Errorf("empty repository")
	}

	state := m.GetState().String()
	description := m.GetDescription()

	return client.Repositories.CreateDeploymentStatus(
		context.Background(),
		repo.GetOwner(),
		repo.GetName(),
		deployment.GetDeploymentID(),
		&gh.DeploymentStatusRequest{
			State:       &state,
			Description: &description,
		},
	)
}
