package deployclient_test

import (
	"testing"

	"github.com/nais/deploy/pkg/deployclient"
	"github.com/stretchr/testify/assert"
)

func TestMultiDocumentParsing(t *testing.T) {
	docs, err := deployclient.MultiDocumentFileAsJSON("testdata/multi_document.yaml", deployclient.TemplateVariables{})
	assert.Len(t, docs, 2)
	assert.NoError(t, err)
	assert.Equal(t, `{"document":1}`, string(docs[0]))
	assert.Equal(t, `{"document":2}`, string(docs[1]))
}

func TestMultiDocumentTemplating(t *testing.T) {
	ctx := deployclient.TemplateVariables{
		"ingresses": []string{
			"https://foo",
			"https://bar",
		},
	}
	docs, err := deployclient.MultiDocumentFileAsJSON("testdata/templating.yaml", ctx)
	assert.Len(t, docs, 2)
	assert.NoError(t, err)
	assert.Equal(t, `{"ingresses":["https://foo","https://bar"]}`, string(docs[0]))
	assert.Equal(t, `{"ungresses":["https://foo","https://bar"]}`, string(docs[1]))
}
