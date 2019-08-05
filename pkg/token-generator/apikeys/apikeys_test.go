package apikeys_test

import (
	"testing"

	"github.com/navikt/deployment/pkg/token-generator/apikeys"
	"github.com/stretchr/testify/assert"
)

func BenchmarkNew(b *testing.B) {
	for n := 0; n < b.N; n++ {
		apikeys.New(32)
	}
}

// Test that API keys can be generated, and are not similar to one another.
func TestNew(t *testing.T) {
	apikey, err := apikeys.New(32)
	assert.NoError(t, err)
	assert.Len(t, apikey, 40) // +25% size base64 encoded

	nextkey, err := apikeys.New(32)
	assert.NoError(t, err)
	assert.Len(t, nextkey, 40)
	assert.NotEqual(t, nextkey, apikey)
}
