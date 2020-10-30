package oauth2

import (
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/oauth2/microsoft"
)

type ClientConfig struct {
	ClientID     string
	ClientSecret string
	TenantID     string
	Scopes       []string
}

func Config(config ClientConfig) clientcredentials.Config {
	return clientcredentials.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Scopes:       config.Scopes,
		TokenURL:     microsoft.AzureADEndpoint(config.TenantID).TokenURL,
	}
}
