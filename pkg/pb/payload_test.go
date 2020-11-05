package pb_test

import (
	"encoding/json"
	"testing"

	"github.com/navikt/deployment/pkg/pb"
	"github.com/stretchr/testify/assert"
)

var payload = []byte(`{ "team": "foo", "kubernetes": { "resources": [ { "foo": "bar", "baz": [564] } ] } }`)

type foostruct struct {
	Foo string `json:"foo"`
	Baz []int  `json:"baz"`
}

func TestJSONPayload(t *testing.T) {
	p, err := pb.PayloadFromJSON(payload)
	assert.NoError(t, err)
	assert.Equal(t, "foo", p.GetTeam())

	js, err := p.JSONResources()
	assert.NoError(t, err)
	assert.Len(t, js, 1)

	fs := &foostruct{}
	err = json.Unmarshal(js[0], fs)
	assert.NoError(t, err)
	assert.Equal(t, "bar", fs.Foo)
	assert.Equal(t, []int{564}, fs.Baz)
}
