package deployclient

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnvStringSlice_StripsSpaces(t *testing.T) {
	const envKey = "TEST_STRING_SLICE"
	os.Setenv(envKey, "file.name, foo.json, bar.com")
	defer os.Unsetenv(envKey)

	got := getEnvStringSlice(envKey)
	want := []string{"file.name", "foo.json", "bar.com"}

	assert.Equal(t, want, got)
}
