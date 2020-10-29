package oauth2

import (
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/oauth2/microsoft"
)

type Client struct {
	ClientID     string
	ClientSecret string
	TenantID     string
}

func (c *Client) Config() clientcredentials.Config {
	return clientcredentials.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Scopes:       []string{"https://graph.microsoft.com/.default"},
		TokenURL:     microsoft.AzureADEndpoint(c.TenantID).TokenURL,
	}

}
