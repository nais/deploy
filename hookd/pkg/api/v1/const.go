package api_v1

const (
	// Maximum time, in seconds, that a request timestamp can differ from the current time.
	MaxTimeSkew = 30.0

	// Length, in bytes, of generated API keys
	KeySize = 32

	SignatureHeader         = "X-NAIS-Signature"
	FailedAuthenticationMsg = "failed authentication"
	DirectDeployGithubTask  = "NAIS_DIRECT_DEPLOY"
)
