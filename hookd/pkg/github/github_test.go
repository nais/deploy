package github_test

import (
	"github.com/navikt/deployment/hookd/pkg/github"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSignatureFromHeader(t *testing.T) {
	t.Run("test that correctly formatted header returns correct data", func(t *testing.T) {
		header := "sha1=6c4f5fc2fbce53aa2011cdf1b2ab37d9dc3b6ecd"
		sig, err := github.SignatureFromHeader(header)
		assert.NoError(t, err)
		assert.Equal(t, []byte{0x6c, 0x4f, 0x5f, 0xc2, 0xfb, 0xce, 0x53, 0xaa, 0x20, 0x11, 0xcd, 0xf1, 0xb2, 0xab, 0x37, 0xd9, 0xdc, 0x3b, 0x6e, 0xcd}, sig)
	})
	t.Run("test unsupported hash function", func(t *testing.T) {
		header := "sha256=6c4f5fc2fbce53aa2011cdf1b2ab37d9dc3b6ecd"
		sig, err := github.SignatureFromHeader(header)
		assert.Error(t, err)
		assert.Nil(t, sig)
	})
	t.Run("test garbage data in hash", func(t *testing.T) {
		header := "sha1=foobar"
		sig, err := github.SignatureFromHeader(header)
		assert.Error(t, err)
		assert.Nil(t, sig)
	})
	t.Run("test missing equal sign in header", func(t *testing.T) {
		header := "sha1"
		sig, err := github.SignatureFromHeader(header)
		assert.Error(t, err)
		assert.Nil(t, sig)
	})
}
