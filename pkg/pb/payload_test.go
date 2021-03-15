package pb_test

import (
	"encoding/json"
	"testing"

	"github.com/nais/deploy/pkg/pb"
	"github.com/stretchr/testify/assert"
)

var resources = []byte(`[ { "foo": "bar", "baz": [564] } ]`)

type foostruct struct {
	Foo string `json:"foo"`
	Baz []int  `json:"baz"`
}

func TestJSONPayload(t *testing.T) {
	p, err := pb.KubernetesFromJSONResources(resources)
	assert.NoError(t, err)

	js, err := p.JSONResources()
	assert.NoError(t, err)
	assert.Len(t, js, 1)

	fs := &foostruct{}
	err = json.Unmarshal(js[0], fs)
	assert.NoError(t, err)
	assert.Equal(t, "bar", fs.Foo)
	assert.Equal(t, []int{564}, fs.Baz)
}
