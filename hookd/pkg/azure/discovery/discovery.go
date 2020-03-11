package discovery

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

var (
	// tokenAuth
	azureCertificates map[string]CertificateList
)

type CertificateList []*x509.Certificate

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

func FetchCertificates() (map[string]CertificateList, error) {
	azure := Azure{
		ClientID:     "ecd35adf-754e-4c75-8098-8e6e1d33cdf9",
		ClientSecret: "x.u5vW3@o]8u5f]jihNsvvgomyCCtw03",
		Tenant:       "62366534-1ec3-4962-8869-9b5535279d0b",
		RedirectURL:  "nothing",
		DiscoveryURL: "https://login.microsoftonline.com/62366534-1ec3-4962-8869-9b5535279d0b/discovery/v2.0/keys",
	}
	if azure.HasConfig() {
		log.Infof("Discover Microsoft signing certificates from %s", azure.DiscoveryURL)
		azureKeyDiscovery, err := DiscoverURL(azure.DiscoveryURL)
		if err != nil {
			return nil, err
		}

		log.Infof("Decoding certificates for %d keys", len(azureKeyDiscovery.Keys))
		azureCertificates, err = azureKeyDiscovery.Map()
		if err != nil {
			return nil, err
		}
	}
	return azureCertificates, nil
}

// Transform a KeyDiscovery object into a dictionary with "kid" as key
// and lists of decoded X509 certificates as values.
//
// Returns an error if any certificate does not decode.
func (k *KeyDiscovery) Map() (result map[string]CertificateList, err error) {
	result = make(map[string]CertificateList)

	for _, key := range k.Keys {
		certList := make(CertificateList, 0)
		for _, encodedCertificate := range key.X5c {
			certificate, err := encodedCertificate.Decode()
			if err != nil {
				return nil, err
			}
			certList = append(certList, certificate)
		}
		result[key.Kid] = certList
	}

	return
}

type KeyDiscovery struct {
	Keys []Key `json:"keys"`
}

func DiscoverURL(url string) (*KeyDiscovery, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	return Discover(response.Body)
}


// Decode a base64 encoded certificate into a X509 structure.
func (c EncodedCertificate) Decode() (*x509.Certificate, error) {
	stream := strings.NewReader(string(c))
	decoder := base64.NewDecoder(base64.StdEncoding, stream)
	key, err := ioutil.ReadAll(decoder)
	if err != nil {
		return nil, err
	}

	return x509.ParseCertificate(key)
}

func Discover(reader io.Reader) (*KeyDiscovery, error) {
	document, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	keyDiscovery := &KeyDiscovery{}
	err = json.Unmarshal(document, keyDiscovery)

	return keyDiscovery, err
}
type EncodedCertificate string

type Key struct {
	Kid string               `json:"kid"`
	X5c []EncodedCertificate `json:"x5c"`
}
