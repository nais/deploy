package apikeys

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// Data store for API keys

var (
	ErrInvalidTeamOrKey = errors.New("team not found or API key invalid")
)

type Source interface {
	Validate(team, key string) error
	Write(team, key string) error
}

func New(randomBytes int) (apiKey string, err error) {
	var n int
	var buf = make([]byte, 0)
	var random = make([]byte, randomBytes)

	n, err = rand.Reader.Read(random)
	if err != nil {
		return
	}
	if n != randomBytes {
		err = fmt.Errorf("not enough entropy")
		return
	}

	w := bytes.NewBuffer(buf)
	encoder := base64.NewEncoder(base64.StdEncoding, w)
	n, err = encoder.Write(random)
	if err != nil {
		return
	}

	return w.String(), nil
}
