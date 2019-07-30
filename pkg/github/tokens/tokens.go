package tokens

import (
	"context"
	"crypto/rsa"
	"time"

	"github.com/dgrijalva/jwt-go"
	gh "github.com/google/go-github/v23/github"
	"golang.org/x/oauth2"
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
