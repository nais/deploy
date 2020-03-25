package database

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	log "github.com/sirupsen/logrus"
)

var (
	ErrNotFound = fmt.Errorf("api key not found")
)

type Database interface {
	Migrate() error
	Read(team string) ([]ApiKey, error)
	ReadAll(team, limit string) ([]ApiKey, error)
	ReadByGroupClaim(group string) ([]ApiKey, error)
	Write(team, groupId string, key []byte) error
	IsErrNotFound(err error) bool

	// legacy layer
	ReadRepositoryTeams(repository string) ([]string, error)
	WriteRepositoryTeams(repository string, teams []string) error
}

type database struct {
	conn          *pgxpool.Pool
	encryptionKey []byte
}

type ApiKey struct {
	Team    string    `json:"team"`
	GroupId string    `json:"groupId"`
	Key     string    `json:"key"`
	Expires time.Time `json:"expires"`
	Created time.Time `json:"created"`
}

var _ Database = &database{}

func New(dsn string, encryptionKey []byte) (Database, error) {
	ctx := context.Background()

	conn, err := pgxpool.Connect(ctx, dsn)
	if err != nil {
		return nil, err
	}

	return &database{
		conn:          conn,
		encryptionKey: encryptionKey,
	}, nil
}

func (db *database) Migrate() error {
	ctx := context.Background()
	var version int

	query := `SELECT MAX(version) FROM migrations`
	row := db.conn.QueryRow(ctx, query)
	err := row.Scan(&version)

	if err != nil {
		// error might be due to no schema.
		// no way to detect this, so log error and continue with migrations.
		log.Warnf("unable to get current migration version: %s", err)
	}

	for version < len(migrations) {
		log.Infof("migrating database schema to version %d", version+1)

		_, err = db.conn.Exec(ctx, migrations[version])
		if err != nil {
			return fmt.Errorf("migrating to version %d: %s", version+1, err)
		}

		version++
	}

	return nil
}

func (db *database) ReadByGroupClaim(group string) ([]ApiKey, error) {
	ctx := context.Background()
	apiKeys := []ApiKey{}

	query := `SELECT key, team, team_azure_id, created, expires FROM apikey WHERE team_azure_id = $1 ORDER BY expires DESC;`
	rows, err := db.conn.Query(ctx, query, group)

	if err != nil {
		return nil, err
	}

	for rows.Next() {
		apiKey := ApiKey{}
		err := rows.Scan(&apiKey.Key, &apiKey.Team, &apiKey.GroupId, &apiKey.Created, &apiKey.Expires)
		if err != nil {
			return nil, err
		}
		apiKeys = append(apiKeys, apiKey)
	}
	return apiKeys, nil
}

func (db *database) Read(team string) ([]ApiKey, error) {
	ctx := context.Background()
	apiKeys := []ApiKey{}

	query := `SELECT key, team, team_azure_id, created, expires FROM apikey WHERE team = $1 AND expires > NOW();`
	rows, err := db.conn.Query(ctx, query, team)

	if err != nil {
		return nil, err
	}

	for rows.Next() {
		apiKey := ApiKey{}
		err := rows.Scan(&apiKey.Key, &apiKey.Team, &apiKey.GroupId, &apiKey.Created, &apiKey.Expires)
		if err != nil {
			return nil, err
		}
		apiKeys = append(apiKeys, apiKey)
	}
	if len(apiKeys) == 0 {
		return nil, ErrNotFound
	}
	return apiKeys, nil
}

func (db *database) ReadAll(team, limit string) ([]ApiKey, error) {
	ctx := context.Background()
	apiKeys := make([]ApiKey, 0)
	var query string
	var rows pgx.Rows
	var err error

	if limit == "" {
		query = `SELECT key, team, team_azure_id, created, expires FROM apikey WHERE team = $1 ORDER BY expires DESC`
		rows, err = db.conn.Query(ctx, query, team)
	} else {
		query = `SELECT key, team, team_azure_id, created, expires FROM apikey WHERE team = $1 ORDER BY expires DESC LIMIT $2`
		rows, err = db.conn.Query(ctx, query, team, limit)
	}

	if err != nil {
		return nil, err
	}

	for rows.Next() {
		apiKey := ApiKey{}
		err := rows.Scan(&apiKey.Key, &apiKey.Team, &apiKey.GroupId, &apiKey.Created, &apiKey.Expires)
		if err != nil {
			return nil, err
		}
		apiKeys = append(apiKeys, apiKey)
	}

	if len(apiKeys) == 0 {
		return nil, ErrNotFound
	}

	return apiKeys, nil
}

func (db *database) Write(team, groupId string, key []byte) error {
	var query string

	ctx := context.Background()

	tx, err := db.conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("unable to start transaction: %s", err)
	}

	query = `UPDATE apikey SET expires = NOW() WHERE expires > NOW() AND team = $1 AND team_azure_id = $2;`
	_, err = tx.Exec(ctx, query, team, groupId)
	if err != nil {
		return err
	}

	query = `
INSERT INTO apikey (key, team, team_azure_id, created, expires)
VALUES ($1, $2, $3, NOW(), NOW()+MAKE_INTERVAL(years := 5));
`
	_, err = tx.Exec(ctx, query, hex.EncodeToString(key), team, groupId)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (db *database) IsErrNotFound(err error) bool {
	return err == ErrNotFound
}

func (db *database) ReadRepositoryTeams(repository string) ([]string, error) {
	ctx := context.Background()

	query := `SELECT team FROM team_repositories WHERE repository = $1;`
	rows, err := db.conn.Query(ctx, query, repository)

	if err != nil {
		return nil, err
	}

	teams := make([]string, 0)
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

func (db *database) WriteRepositoryTeams(repository string, teams []string) error {
	var query string

	ctx := context.Background()

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
