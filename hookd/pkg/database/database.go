package database

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
)

var (
	ErrNotFound = fmt.Errorf("api key not found")
)

const (
	NotFoundMessage = "The specified key does not exist."
)

type Database interface {
	Migrate() error
	Read(team string) ([][]byte, error)
	Write(team string, key []byte) error
	IsErrNotFound(err error) bool
}

type database struct {
	conn *pgx.Conn
}

var _ Database = &database{}

func New(dsn string) (Database, error) {
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, err
	}

	return &database{
		conn: conn,
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

func (db *database) Read(team string) ([][]byte, error) {
	var key string
	keys := make([][]byte, 0)
	ctx := context.Background()

	query := `SELECT key FROM apikey WHERE team = $1 AND expires > NOW();`
	rows, err := db.conn.Query(ctx, query, team)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		err = rows.Scan(&key)
		if err != nil {
			return nil, err
		}
		decoded, err := hex.DecodeString(key)
		if err != nil {
			return nil, err
		}
		keys = append(keys, decoded)
	}

	if len(keys) == 0 {
		return nil, ErrNotFound
	}

	return keys, nil
}

func (db *database) Write(team string, key []byte) error {
	var query string

	ctx := context.Background()

	tx, err := db.conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("unable to start transaction: %s", err)
	}

	query = `UPDATE apikey SET expires = NOW() WHERE expires > NOW() AND team = $1;`
	_, err = tx.Exec(ctx, query, team)
	if err != nil {
		return err
	}

	query = `
INSERT INTO apikey (key, team, created, expires)
VALUES ($2, $1, NOW(), NOW()+MAKE_INTERVAL(years := 5));
`
	_, err = tx.Exec(ctx, query, team, hex.EncodeToString(key))
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (db *database) IsErrNotFound(err error) bool {
	return err == ErrNotFound
}
