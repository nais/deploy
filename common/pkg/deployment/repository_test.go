package deployment_test

import (
	"testing"

	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/stretchr/testify/assert"
)

func TestGithubRepository_FullName(t *testing.T) {
	repo := deployment.GithubRepository{
		Owner: "foo",
		Name:  "bar",
	}
	assert.Equal(t, "foo/bar", repo.FullName())
}
