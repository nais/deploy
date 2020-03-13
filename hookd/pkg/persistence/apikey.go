package persistence

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/jackc/pgx/v4"
)

var (
	ErrNotFound = fmt.Errorf("api key not found")
)

const (
	NotFoundMessage = "The specified key does not exist."
)

type ApiKeyStorage interface {
	Read(team string) ([][]byte, error)
	Write(team string, key []byte) error
	IsErrNotFound(err error) bool
}

type PostgresApiKeyStorage struct {
	ConnectionString string
}

var _ ApiKeyStorage = &PostgresApiKeyStorage{}

func (s *PostgresApiKeyStorage) Read(team string) ([][]byte, error) {
	var key string
	keys := make([][]byte, 0)
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, s.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %s", err)
	}
	defer conn.Close(ctx)

	query := `SELECT key FROM apikey WHERE team = $1 AND expires > NOW();`
	rows, err := conn.Query(ctx, query, team)
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

func (s *PostgresApiKeyStorage) Write(team string, key []byte) error {
	var query string

	ctx := context.Background()

	conn, err := pgx.Connect(ctx, s.ConnectionString)
	if err != nil {
		return fmt.Errorf("unable to connect to database: %s", err)
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
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

func (s *PostgresApiKeyStorage) IsErrNotFound(err error) bool {
	return err == ErrNotFound
}
