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

type MockApiKeyStorage struct{}

func (a *MockApiKeyStorage) Read(team string) ([]byte, error) {
	return []byte("1234"), nil
}

func (a *MockApiKeyStorage) IsErrNotFound(err error) bool {
	return true
}
