package apikeys

import (
	"errors"
)

// Data store for API keys

var (
	ErrInvalidTeamOrKey = errors.New("team not found or API key invalid")
)

type Source interface {
	Validate(team, key string) error
	Write(team, key string) error
}
