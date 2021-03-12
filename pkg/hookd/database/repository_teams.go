package database

import (
	"context"
	"fmt"
)

// legacy layer
type RepositoryTeamStore interface {
	ReadRepositoryTeams(ctx context.Context, repository string) ([]string, error)
	WriteRepositoryTeams(ctx context.Context, repository string, teams []string) error
}

var _ RepositoryTeamStore = &Database{}

func (db *Database) ReadRepositoryTeams(ctx context.Context, repository string) ([]string, error) {
	query := `SELECT team FROM team_repositories WHERE repository = $1;`
	rows, err := db.timedQuery(ctx, query, repository)

	if err != nil {
		return nil, err
	}

	teams := make([]string, 0)

	defer rows.Close()
	for rows.Next() {
		var team string
		err := rows.Scan(&team)
		if err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}

	if len(teams) == 0 {
		return nil, ErrNotFound
	}

	return teams, nil
}

func (db *Database) WriteRepositoryTeams(ctx context.Context, repository string, teams []string) error {
	var query string

	tx, err := db.conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("unable to start transaction: %s", err)
	}

	query = `DELETE FROM team_repositories WHERE repository = $1;`
	_, err = tx.Exec(ctx, query, repository)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	for _, team := range teams {
		query = `INSERT INTO team_repositories (team, repository) VALUES ($1, $2);`
		_, err = tx.Exec(ctx, query, team, repository)
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
	}

	return tx.Commit(ctx)
}
