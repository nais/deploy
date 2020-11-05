package pb_test

import (
	"testing"

	"github.com/navikt/deployment/pkg/pb"
	"github.com/stretchr/testify/assert"
)

func TestGithubRepository_FullName(t *testing.T) {
	repo := pb.GithubRepository{
		Owner: "foo",
		Name:  "bar",
	}
	assert.Equal(t, "foo/bar", repo.FullName())
}
