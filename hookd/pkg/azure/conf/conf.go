package conf

type Azure struct {
	ClientID     string `json:"clientid"`
	ClientSecret string `json:"clientsecret"`
	Tenant       string `json:"tenant"`
	RedirectURL  string `json:"redirecturl"`
	DiscoveryURL string `json:"discoveryurl"`
}
