package persistence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/minio/minio-go"
	"github.com/navikt/deployment/hookd/pkg/config"
	log "github.com/sirupsen/logrus"
)

var (
	ErrNotFound = fmt.Errorf("path not found")
)

const (
	notFoundMessage = "The specified key does not exist."
)

type ApiKeyStorage interface {
	Read(team string) ([]byte, error)
	IsErrNotFound(err error) bool
}
