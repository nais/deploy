package apikeys

import (
	"golang.org/x/crypto/bcrypt"
)

var _ Source = &memoryStore{}

type memoryStore struct {
	keys map[string]string
}

func NewMemoryStore() *memoryStore {
	return &memoryStore{
		keys: make(map[string]string),
	}
}

func (m *memoryStore) Write(team, key string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	m.keys[team] = string(hash)
	return nil
}

func (m *memoryStore) Validate(team, key string) error {
	if _, ok := m.keys[team]; !ok {
		return ErrInvalidTeamOrKey
	}
	err := bcrypt.CompareHashAndPassword([]byte(m.keys[team]), []byte(key))
	if err != nil {
		return ErrInvalidTeamOrKey
	}
	return nil
}
