package secrets

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	vault "github.com/hashicorp/vault/api"
	"strings"
	"time"
)

// Client stores secrets in a Hashicorp Vault backend.
type Client struct {
	vaultClient *vault.Logical
	path        string
}

type InstallationSecret struct {
	Repository     string
	InstallationID string
	WebhookSecret  string
}

const (
	installationIDKey = "installationID"
	webhookSecretKey  = "webhookSecret"
)

func RandomString(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func tostring(x interface{}) (string, error) {
	switch s := x.(type) {
	case string:
		return s, nil
	default:
		return "", fmt.Errorf("cannot convert secret data to string format")
	}
}

func New(address, token, path string) (*Client, error) {
	cf := vault.DefaultConfig()
	cf.Address = address
	cf.Timeout = time.Second * 2
	client, err := vault.NewClient(cf)
	if err != nil {
		return nil, err
	}
	client.SetToken(token)
	return &Client{
		vaultClient: client.Logical(),
		path:        strings.TrimRight(path, "/"),
	}, nil
}

func (c *Client) mkpath(path string) string {
	return c.path + "/" + strings.Trim(path, "/")
}

// Return the secret for the "NAV deployment" Github application.
func (c *Client) ApplicationSecret() (string, error) {
	path := c.mkpath("/_application")
	secret, err := c.vaultClient.Read(path)
	if err != nil {
		return "", err
	}
	return tostring(secret.Data[webhookSecretKey])
}

// Given a repository name in the form ORGANIZATION/NAME, return the pre-shared webhook secret.
func (c *Client) InstallationSecret(repository string) (InstallationSecret, error) {
	is := InstallationSecret{}
	path := c.mkpath(repository)
	secret, err := c.vaultClient.Read(path)
	if err != nil {
		return is, err
	}
	is.Repository = repository
	is.InstallationID, err = tostring(secret.Data[installationIDKey])
	if err != nil {
		return is, err
	}
	is.WebhookSecret, err = tostring(secret.Data[webhookSecretKey])
	if err != nil {
		return is, err
	}
	return is, nil
}

func (c *Client) WriteInstallationSecret(s InstallationSecret) error {
	path := c.mkpath(s.Repository)
	payload := map[string]interface{}{
		installationIDKey: s.InstallationID,
		webhookSecretKey:  s.WebhookSecret,
	}
	_, err := c.vaultClient.Write(path, payload)
	return err
}
