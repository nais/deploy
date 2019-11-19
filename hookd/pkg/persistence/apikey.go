package persistence

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/navikt/deployment/hookd/pkg/metrics"
	log "github.com/sirupsen/logrus"
)

var (
	ErrNotFound = fmt.Errorf("api key not found")
)

const (
	NotFoundMessage          = "The specified key does not exist."
	RefreshIntervalFactor    = 0.8
	InitialTokenWaitDuration = 1 * time.Millisecond
)

type ApiKeyStorage interface {
	Read(team string) ([]byte, error)
	Write(team string, key []byte) error
	IsErrNotFound(err error) bool
}

type VaultApiKeyStorage struct {
	Address       string
	Path          string
	AuthPath      string
	AuthRole      string
	KeyName       string
	Credentials   string
	Token         string
	LeaseDuration int
	HttpClient    *http.Client
}

type VaultResponse struct {
	Data map[string]string `json:"data"`
}

type VaultAuthRequest struct {
	JWT  string `json:"jwt"`
	Role string `json:"role"`
}

type VaultAuthResponse struct {
	Auth struct {
		ClientToken   string `json:"client_token"`
		LeaseDuration int    `json:"lease_duration"`
	} `json:"auth"`
}

type VaultWriteRequest struct {
	Key string `json:"key"`
}

func (s *VaultApiKeyStorage) refreshToken() error {
	u, err := url.Parse(s.Address)

	if err != nil {
		return fmt.Errorf("unable to construct auth URL: %s", err)
	}

	u.Path = s.AuthPath
	b, err := json.Marshal(VaultAuthRequest{JWT: s.Credentials, Role: s.AuthRole})

	if err != nil {
		return fmt.Errorf("unable to marshal auth request: %s", err)
	}

	resp, err := http.Post(u.String(), "application/json", bytes.NewReader(b))
	if resp != nil {
		metrics.VaultTokenRefresh.WithLabelValues(strconv.Itoa(resp.StatusCode)).Inc()
	}

	if err != nil {
		return fmt.Errorf("unable to perform auth request: %s", err)
	}

	var vaultAuthResponse VaultAuthResponse

	if err := json.NewDecoder(resp.Body).Decode(&vaultAuthResponse); err != nil {
		return fmt.Errorf("unable to decode auth response: %s", err)
	}

	s.Token = vaultAuthResponse.Auth.ClientToken

	s.LeaseDuration = vaultAuthResponse.Auth.LeaseDuration
	log.Debugf("Vault: token expires in %d seconds", s.LeaseDuration)

	return nil
}

func (s *VaultApiKeyStorage) RefreshLoop() {
	timer := time.NewTimer(InitialTokenWaitDuration)

	for range timer.C {
		if err := s.refreshToken(); err != nil {
			log.Errorf("Unable to refresh Vault token: %s", err)
			timer.Reset(1 * time.Minute)
			continue
		}
		refreshInterval := int(float64(s.LeaseDuration) * RefreshIntervalFactor)
		duration := time.Duration(refreshInterval) * time.Second
		timer.Reset(duration)
		log.Debugf("Successfully refreshed Vault token, next refresh in %s", duration.String())
	}
}

func (s *VaultApiKeyStorage) Read(team string) ([]byte, error) {
	u, err := url.Parse(s.Address)

	if err != nil {
		return nil, fmt.Errorf("unable to construct URL to Vault: %s", err)
	}

	u.Path = path.Join(s.Path, team)

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)

	if err != nil {
		return nil, fmt.Errorf("unable to create HTTP request: %s", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", s.Token))

	resp, err := s.HttpClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("unable to get key from Vault: %s", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var vaultResp VaultResponse
		decoder := json.NewDecoder(resp.Body)
		defer resp.Body.Close()
		if err := decoder.Decode(&vaultResp); err != nil {
			return nil, fmt.Errorf("unable to unmarshal response from Vault: %s", err)
		}

		return hex.DecodeString(vaultResp.Data[s.KeyName])
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("Vault returned HTTP %d: %s", resp.StatusCode, string(body))
	}
}

func (s *VaultApiKeyStorage) Write(team string, key []byte) error {
	u, err := url.Parse(s.Address)

	if err != nil {
		return fmt.Errorf("unable to construct URL to Vault: %s", err)
	}

	u.Path = path.Join(s.Path, team)

	writeRequest := &VaultWriteRequest{
		Key: hex.EncodeToString(key),
	}
	payload, err := json.Marshal(writeRequest)
	if err != nil {
		return fmt.Errorf("create api key payload: %s", err)
	}

	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(payload))

	if err != nil {
		return fmt.Errorf("unable to create HTTP request: %s", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", s.Token))
	req.Header.Set("content-type", "application/json")

	resp, err := s.HttpClient.Do(req)

	if err != nil {
		return fmt.Errorf("write team API key to Vault: %s", err)
	} else if resp != nil && resp.StatusCode >= 400 {
		errmsg, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("write team API key to Vault: %s; %s", resp.Status, errmsg)
	}

	log.Infof("Wrote Vault team API key for team '%s', response was %s", team, resp.Status)

	return nil
}

func (s *VaultApiKeyStorage) IsErrNotFound(err error) bool {
	return err == ErrNotFound
}

type StaticKeyApiKeyStorage struct {
	Key []byte
}

func (s *StaticKeyApiKeyStorage) Read(team string) ([]byte, error) {
	return s.Key, nil
}

func (s *StaticKeyApiKeyStorage) IsErrNotFound(err error) bool {
	return true
}
