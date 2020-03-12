package conf

type Azure struct {
	ClientID     string `json:"clientid"`
	ClientSecret string `json:"clientsecret"`
	Tenant       string `json:"tenant"`
	RedirectURL  string `json:"redirecturl"`
	DiscoveryURL string `json:"discoveryurl"`
}

func (a *Azure) HasConfig() bool {
	return a.ClientID != "" &&
		a.ClientSecret != "" &&
		a.Tenant != "" &&
		a.RedirectURL != "" &&
		a.DiscoveryURL != ""
}
