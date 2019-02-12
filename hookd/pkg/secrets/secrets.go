package secrets

import (
	"crypto/rand"
	"encoding/hex"
)

type VaultData struct {
	Id       int
	FullName string
	Secret   string
}

const GithubPreSharedKey = "BxVAH2dVbbvawyFkDD3L8JLUHzMEFQQlu9YCqNq0R7BEdragxICFJtr4jJZYBbXs"

// Return the secret for the "NAV deployment" Github application.
func ApplicationWebhookSecret() (string, error) {
	return GithubPreSharedKey, nil
}

// Given a repository name in the form ORGANIZATION/NAME, return the pre-shared webhook secret.
func RepositoryWebhookSecret(repository string) (string, error) {
	return GithubPreSharedKey, nil
}

func RandomString(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
