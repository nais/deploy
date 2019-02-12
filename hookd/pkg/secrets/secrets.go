package secrets

const GithubPreSharedKey = "BxVAH2dVbbvawyFkDD3L8JLUHzMEFQQlu9YCqNq0R7BEdragxICFJtr4jJZYBbXs"

func GlobalApplicationSecret() (string, error) {
	return GithubPreSharedKey, nil
}

func RepositorySecret(repository string) (string, error) {
	return GithubPreSharedKey, nil
}
