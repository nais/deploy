package tokens_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/navikt/deployment/pkg/github/tokens"
	"github.com/stretchr/testify/assert"
)

const (
	appID    = "12345"
	duration = time.Second * 1337
)

// Test generation of a signed JSON Web Token
func TestNew(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil || key == nil {
		panic(err)
	}

	pemPublicKey := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&key.PublicKey),
	})
	pemPrivateKey := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	now := float64(time.Now().Unix())

	signed, err := tokens.AppToken(key, appID, duration)
	assert.NoError(t, err)
	assert.True(t, len(signed) > 5)

	fmt.Println("*********************************")
	fmt.Println("Private key used to generate token:")
	fmt.Println(string(pemPrivateKey))
	fmt.Println("Public key for token validation:")
	fmt.Println(string(pemPublicKey))
	fmt.Println("Signed token:")
	fmt.Println(signed)
	fmt.Println("*********************************")

	token, err := jwt.Parse(signed, func(token *jwt.Token) (interface{}, error) {
		return &key.PublicKey, nil
	})
	assert.NoError(t, err)
	assert.True(t, token.Valid)

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		assert.Equal(t, appID, claims["iss"])
		assert.True(t, claims["iat"].(float64)-now == 0)
		assert.True(t, claims["exp"].(float64)-now-duration.Seconds() == 0)
	}
}
