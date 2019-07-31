package github_source

import (
	"context"
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/dgrijalva/jwt-go"
	gh "github.com/google/go-github/v27/github"
	"github.com/navikt/deployment/pkg/token-generator/types"
	"golang.org/x/oauth2"
)

type InstallationTokenRequest struct {
	Key            *rsa.PrivateKey
	ApplicationID  string
	InstallationID int64
}

const (
	appTokenValidity = time.Second * 30
)

// Creates a new GitHub App JWT, signed with the specified key and
// encoded using the RS256 algorithm. This key can not be used against repositories.
//
// See https://developer.github.com/apps/building-github-apps/authenticating-with-github-apps/#authenticating-as-a-github-app
func AppToken(key *rsa.PrivateKey, appID string, duration time.Duration) (string, error) {
	claims := jwt.StandardClaims{
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(duration).Unix(),
		Issuer:    appID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	return token.SignedString(key)
}

// Use a GitHub App JWT to create an Installation Token,
// which can be used to perform operations against repositories.
//
// See https://developer.github.com/v3/apps/#create-a-new-installation-token
//
// FIXME: scoped tokens are not available in the upstream API, but there is an open PR:
// FIXME: https://github.com/google/go-github/issues/1237
// FIXME: https://github.com/google/go-github/pull/1238
func InstallationToken(appToken string, installationID int64) (string, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: appToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := gh.NewClient(tc)

	token, _, err := client.Apps.CreateInstallationToken(ctx, installationID)

	return token.GetToken(), err
}

func Credentials(request InstallationTokenRequest) (*types.Credentials, error) {
	appToken, err := AppToken(request.Key, request.ApplicationID, appTokenValidity)
	if err != nil {
		return nil, fmt.Errorf("generate app token: %s", err)
	}

	installationToken, err := InstallationToken(appToken, request.InstallationID)
	if err != nil {
		return nil, fmt.Errorf("generate installation token: %s", err)
	}

	return &types.Credentials{
		Token: installationToken,
	}, nil
}

func RSAPrivateKeyFromPEMFile(filename string) (*rsa.PrivateKey, error) {
	keyBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read private key: %s", err)
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %s", err)
	}

	// Check that creation of a single token succeeds. If it doesn't, there is
	// a high chance that we can't sign any tokens at all.
	_, err = AppToken(key, "", time.Second)
	if err != nil {
		return nil, fmt.Errorf("token generation with private key: %s", err)
	}

	return key, nil
}
