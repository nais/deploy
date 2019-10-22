package persistence

import (
	"fmt"
)

var (
	ErrNotFound = fmt.Errorf("path not found")
)

const (
	NotFoundMessage = "The specified key does not exist."
)

type ApiKeyStorage interface {
	Read(team string) ([]byte, error)
	IsErrNotFound(err error) bool
}

type StaticKeyApiKeyStorage struct {
	Key []byte
}

func (a *StaticKeyApiKeyStorage) Read(team string) ([]byte, error) {
	return a.Key, nil
}

func (a *StaticKeyApiKeyStorage) IsErrNotFound(err error) bool {
	return true
}
