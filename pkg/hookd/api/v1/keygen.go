package api_v1

import (
	"crypto/rand"
)

// Generate a cryptographically secure random key of N length.
func Keygen(length int) (Key, error) {
	buf := make([]byte, length)
	_, err := rand.Read(buf)
	return buf, err
}
