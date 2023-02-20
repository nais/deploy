package deployclient

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nais/deploy/pkg/version"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DeployClientVersion  = "deploy.nais.io/client-version"
	GithubWorkflowRunURL = "deploy.nais.io/github-workflow-run-url"
)

func InjectAnnotations(resource json.RawMessage, annotations map[string]string) (json.RawMessage, error) {
	decoded := make(map[string]json.RawMessage)
	err := json.Unmarshal(resource, &decoded)
	if err != nil {
		return nil, err
	}

	meta := &v1.ObjectMeta{}
	err = json.Unmarshal(decoded["metadata"], meta)
	if err != nil {
		return nil, fmt.Errorf("error in metadata field: %w", err)
	}

	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		meta.Annotations[k] = v
	}
	encoded, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	decoded["metadata"] = encoded
	return json.Marshal(decoded)
}

// https://docs.github.com/en/actions/reference/environment-variables#default-environment-variables
func BuildEnvironmentAnnotations() map[string]string {
	a := make(map[string]string)

	add := func(envVar, key string) {
		value, found := os.LookupEnv(envVar)
		if found {
			a[key] = value
		}
	}
	addAll := func(envVar ...string) {
		for _, v := range envVar {
			key := "deploy.nais.io/" + strings.ReplaceAll(strings.ToLower(v), "_", "-")
			add(v, key)
		}
	}

	addAll(
		// GitHub
		"GITHUB_ACTOR",
		"GITHUB_SHA",

		// Jenkins
		"BUILD_URL",
		"GIT_COMMIT",
	)

	a[DeployClientVersion] = version.Version()
	runurl := githubWorkflowRunURL()
	if len(runurl) > 0 {
		a[GithubWorkflowRunURL] = runurl
	}

	cause := changeCause(a)
	if len(cause) > 0 {
		a["kubernetes.io/change-cause"] = cause
	}

	return a
}

func changeCause(annotations map[string]string) string {
	var commit, url string
	var ok bool

	for _, key := range []string{"deploy.nais.io/github-sha", "deploy.nais.io/git-commit"} {
		commit, ok = annotations[key]
		if ok {
			break
		}
	}

	for _, key := range []string{GithubWorkflowRunURL, "deploy.nais.io/build-url"} {
		url, ok = annotations[key]
		if ok {
			break
		}
	}

	if len(commit) == 0 || len(url) == 0 {
		return ""
	}

	return fmt.Sprintf("nais deploy: commit %s: %s", commit, url)
}

func githubWorkflowRunURL() string {
	server, ok := os.LookupEnv("GITHUB_SERVER_URL")
	if !ok {
		return ""
	}
	repo, ok := os.LookupEnv("GITHUB_REPOSITORY")
	if !ok {
		return ""
	}
	runid, ok := os.LookupEnv("GITHUB_RUN_ID")
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s/%s/actions/runs/%s", server, repo, runid)
}
