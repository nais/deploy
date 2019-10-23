package persistence

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
)

var (
	ErrNotFound = fmt.Errorf("api key not found")
)

const (
	NotFoundMessage = "The specified key does not exist."
)

type ApiKeyStorage interface {
	Read(team string) ([]byte, error)
	IsErrNotFound(err error) bool
}

type VaultApiKeyStorage struct {
	Address    string
	Path       string
	KeyName    string
	Token      string
	HttpClient *http.Client
}

type VaultResponse struct {
	Data map[string]string `json:"data"`
}

func (s *VaultApiKeyStorage) Read(team string) ([]byte, error) {
	u, err := url.Parse(s.Address)
	u.Path = path.Join(s.Path, team)

	if err != nil {
		return nil, fmt.Errorf("unable to construct URL to vault: %s", err)
	}

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
			return nil, fmt.Errorf("unable to unmarshal response from vault: %s", err)
		}

		return []byte(vaultResp.Data[s.KeyName]), err
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		return nil, fmt.Errorf("unsuccessful http status code from vault: %d", resp.StatusCode)
	}
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
