package github

type Payload struct {
	Version  [3]int
	NaisYaml string
}

type Deployment struct {
	Id          int
	Payload     Payload
	Sha         string
	Environment string
}

type Repository struct {
	FullName string `json:"full_name"`
}

// Standalone payload from Github
type DeploymentRequest struct {
	Deployment Deployment
	Repository Repository
}

// Standalone payload from Github
type IntegrationInstallation struct {
	Action       string
	Repositories []Repository
}

type Webhook struct {
	ID     int           `json:"id",omit_empty`
	Name   string        `json:"name"`
	Config WebhookConfig `json:"config"`
	Events []string      `json:"events"`
	Active bool          `json:"active"`
}

type WebhookConfig struct {
	Url         string `json:"url"`
	ContentType string `json:"content_type"`
	Secret      string `json:"secret"`
	InsecureSSL string `json:"insecure_ssl"`
}
