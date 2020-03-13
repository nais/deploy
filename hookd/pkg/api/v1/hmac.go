package api_v1

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
)

// ValidateMAC reports whether messageMAC is a valid HMAC tag for message.
func ValidateMAC(message, messageMAC, key []byte) bool {
	expectedMAC := GenMAC(message, key)
	return hmac.Equal(messageMAC, expectedMAC)
}

// GenMAC generates the HMAC signature for a message provided the secret key using SHA256
func GenMAC(message, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	return mac.Sum(nil)
}

func ValidateAnyMAC(message, messageMAC []byte, keys [][]byte) error {
	for _, key := range keys {
		if ValidateMAC(message, messageMAC, key) {
			return nil
		}
	}
	return fmt.Errorf("%s: HMAC signature error", FailedAuthenticationMsg)
}
