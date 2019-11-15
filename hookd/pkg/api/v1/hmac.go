package api_v1

import (
	"crypto/hmac"
	"crypto/sha256"
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
