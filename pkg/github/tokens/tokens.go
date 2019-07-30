package tokens

import (
	"crypto/rsa"
	"time"

	"github.com/dgrijalva/jwt-go"
)

// Creates a new GitHub App JWT, signed with the specified key and
// encoded using the RS256 algorithm.
//
// See https://developer.github.com/apps/building-github-apps/authenticating-with-github-apps/#authenticating-as-a-github-app
func New(key *rsa.PrivateKey, appID string, duration time.Duration) (string, error) {
	claims := jwt.StandardClaims{
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(duration).Unix(),
		Issuer:    appID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	return token.SignedString(key)
}
