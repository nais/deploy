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
