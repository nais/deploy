package azure

import (
	"net/url"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// Returns oauth2 config that can be used in auth code flow.
func NewUserConfig(clientID, clientSecret, tenant, redirectURL string) oauth2.Config {
	return oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:   AuthorizeURL(tenant, "authorize"),
			TokenURL:  AuthorizeURL(tenant, "token"),
			AuthStyle: oauth2.AuthStyleInParams,
		},
		Scopes: []string{
			"Group.Read.All",
			"User.Read",
		},
	}
}

// Returns oauth2 config that is used in client credentials flow.
func NewApplicationConfig(clientID, clientSecret, tenant string) clientcredentials.Config {
	endpointParams := url.Values{}
	endpointParams.Set("resource", "https://graph.microsoft.com")

	return clientcredentials.Config{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		TokenURL:       AuthorizeURL(tenant, "token"),
		EndpointParams: endpointParams,
		Scopes: []string{
			"Group.Read.All",
			"User.Read",
		},
	}
}
