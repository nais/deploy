package github

import (
	"encoding/hex"
	"fmt"
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
