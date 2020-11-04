package discovery

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/navikt/deployment/hookd/pkg/config"
	log "github.com/sirupsen/logrus"
)

type OpenIDConfiguration struct {
	JwksURI string `json:"jwks_uri"`
}

type CertificateList []*x509.Certificate

func FetchCertificates(azure config.Azure) (map[string]CertificateList, error) {
	log.Infof("Discover OpenID configuration from %s", azure.WellKnownURL)
	openIDConfig, err := GetOpenIDConfiguration(azure.WellKnownURL)
	if err != nil {
		return nil, err
	}

	log.Infof("Discover signing certificates from %s", openIDConfig.JwksURI)
	azureKeyDiscovery, err := DiscoverURL(openIDConfig.JwksURI)
	if err != nil {
		return nil, err
	}

	log.Infof("Decoding certificates for %d keys", len(azureKeyDiscovery.Keys))
	azureCertificates, err := azureKeyDiscovery.Map()
	if err != nil {
		return nil, err
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

func GetOpenIDConfiguration(url string) (*OpenIDConfiguration, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	cfg := &OpenIDConfiguration{}
	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
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
