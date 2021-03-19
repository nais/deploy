package deployclient_test

import (
	"encoding/json"
	"testing"

	"github.com/nais/deploy/pkg/deployclient"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestInjectAnnotations(t *testing.T) {
	docs, err := deployclient.MultiDocumentFileAsJSON("testdata/nais.yaml", nil)
	assert.NoError(t, err)
	assert.Len(t, docs, 1)

	// Check that unmodified application contains the sample annotation
	app := &nais_io_v1alpha1.Application{}
	err = json.Unmarshal(docs[0], app)
	assert.NoError(t, err)
	assert.EqualValues(t, map[string]string{"some-annotation": "yes"}, app.GetAnnotations())

	annotations := map[string]string{
		"foo": "bar",
		"yes": "no",
	}

	// Merge our custom annotations
	modified, err := deployclient.InjectAnnotations(docs[0], annotations)
	assert.NoError(t, err)

	// Check that the resulting object contains all three annotations
	annotations["some-annotation"] = "yes"
	app = &nais_io_v1alpha1.Application{}
	err = json.Unmarshal(modified, app)
	assert.NoError(t, err)

	assert.EqualValues(t, annotations, app.GetAnnotations())
}
