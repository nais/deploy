package github

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/bradleyfalzon/ghinstallation"
	gh "github.com/google/go-github/v23/github"
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

func ApplicationClient(appId int, keyFile string) (*gh.Client, error) {
	itr, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, appId, keyFile)
	if err != nil {
		return nil, err
	}
	return gh.NewClient(&http.Client{Transport: itr}), nil
}

func InstallationClient(appId, installId int, keyFile string) (*gh.Client, error) {
	itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appId, installId, keyFile)
	if err != nil {
		return nil, err
	}
	return gh.NewClient(&http.Client{Transport: itr}), nil
}

func CreateHook(client *gh.Client, r gh.Repository, url string) (*gh.Hook, error) {
	secret, err := secrets.RandomString(32)
	if err != nil {
		return nil, err
	}

	active := true
	webhook := &gh.Hook{
		Events: []string{
			"deployment",
		},
		Active: &active,
		Config: map[string]interface{}{
			"url":          url,
			"content_type": "json",
			"insecure_ssl": "0",
			"secret":       secret,
		},
	}

	fullName := r.GetFullName()
	parts := strings.Split(fullName, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("repository name is not in the format OWNER/NAME")
	}
	webhook, _, err = client.Repositories.CreateHook(context.Background(), parts[0], parts[1], webhook)
	if err != nil {
		return nil, err
	}

	/*
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("webhook creation returned status code %d, expected %d", resp.StatusCode, http.StatusCreated)
	}

	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("while decoding server response: %s", err)
	}
	*/

	log.Infof("oops, webhook secret for %s is %s", r.GetFullName(), secret)
	return webhook, nil
}
