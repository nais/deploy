package github

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/navikt/deployment/hookd/pkg/secrets"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
)

// SignatureFromHeader takes a header string containing a hash format
// and a hash value, and returns the hash value as a byte array.
//
// Example data: sha1=6c4f5fc2fbce53aa2011cdf1b2ab37d9dc3b6ecd
func SignatureFromHeader(header string) ([]byte, error) {
	parts := strings.SplitN(header, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("wrong format for hash, expected 'sha1=hash', got '%s'", header)
	}
	if parts[0] != "sha1" {
		return nil, fmt.Errorf("expected hash type 'sha1', got '%s'", parts[0])
	}
	hexSignature, err := hex.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("error in hexadecimal format '%s': %s", parts[1], err)
	}
	return hexSignature, nil
}

func CreateHook(r Repository, url string) (*Webhook, error) {
	// https://developer.github.com/v3/repos/hooks/#create-a-hook
	secret, err := secrets.RandomString(32)
	if err != nil {
		return nil, err
	}

	webhook := Webhook{
		Name: "web",
		Events: []string{
			"deployment",
		},
		Active: true,
		Config: WebhookConfig{
			Url:         url,
			ContentType: "json",
			InsecureSSL: "0",
			Secret:      secret,
		},
	}

	b, err := json.Marshal(webhook)
	if err != nil {
		return nil, fmt.Errorf("while marshalling webhook to JSON: %s", err)
	}
	reader := bytes.NewReader(b)

	webhookUrl := fmt.Sprintf("/repos/%s/hooks", r.FullName)
	c := http.Client{}
	resp, err := c.Post(webhookUrl, "application/json", reader)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("webhook creation returned status code %d, expected %d", resp.StatusCode, http.StatusCreated)
	}

	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("while decoding server response: %s", err)
	}

	log.Infof("oops, webhook secret for %s is %s", r.FullName, webhook.Config.Secret)
	return &webhook, nil
}
