package persistence

import (
	"github.com/navikt/deployment/hookd/pkg/database"
)

type TeamRepositoryStorage interface {
	Read(repository string) ([]string, error)
	Write(repository string, teams []string) error
	IsErrNotFound(err error) bool
}

type trs struct {
	db database.Database
}

func NewTeamRepositoryStorage(db database.Database) TeamRepositoryStorage {
	return &trs{
		db: db,
	}
}

func (s *trs) Read(repository string) ([]string, error) {
	return s.db.ReadRepositoryTeams(repository)
}

func (s *trs) Write(repository string, teams []string) error {
	return s.db.WriteRepositoryTeams(repository, teams)
}

func (s *trs) IsErrNotFound(err error) bool {
	return s.db.IsErrNotFound(err)
}
