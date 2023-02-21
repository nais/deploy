package database

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/nais/deploy/pkg/crypto"
	api_v1 "github.com/nais/deploy/pkg/hookd/api/v1"
)

type ApiKey struct {
	Team    string     `json:"team"`
	Key     api_v1.Key `json:"key"`
	Expires time.Time  `json:"expires"`
	Created time.Time  `json:"created"`
}

type ApiKeyStore interface {
	ApiKeys(ctx context.Context, id string) (ApiKeys, error)
	RotateApiKey(ctx context.Context, team string, key api_v1.Key) error
}

var _ ApiKeyStore = &Database{}

type ApiKeys []ApiKey

func (apikeys ApiKeys) Keys() []api_v1.Key {
	keys := make([]api_v1.Key, len(apikeys))
	for i := range apikeys {
		keys[i] = apikeys[i].Key
	}
	return keys
}

func (apikeys ApiKeys) Valid() ApiKeys {
	valid := make(ApiKeys, 0, len(apikeys))
	for _, apikey := range apikeys {
		if apikey.Expires.After(time.Now()) {
			valid = append(valid, apikey)
		}
	}
	return valid
}

func (apikeys ApiKeys) ValidKeys() []api_v1.Key {
	keys := make([]api_v1.Key, 0, len(apikeys))
	for _, apiKey := range apikeys.Valid() {
		if len(apiKey.Key) > 0 {
			keys = append(keys, apiKey.Key)
		}
	}
	return keys
}

const selectApiKeyFields = `key, team, created, expires`

func (db *Database) decrypt(encrypted string) ([]byte, error) {
	decoded, err := hex.DecodeString(encrypted)
	if err != nil {
		return nil, fmt.Errorf("decode hex: %s", err)
	}
	return crypto.Decrypt(decoded, db.encryptionKey)
}

func (db *Database) scanApiKeyRows(rows pgx.Rows) (ApiKeys, error) {
	apiKeys := make(ApiKeys, 0)

	defer rows.Close()
	for rows.Next() {
		var apiKey ApiKey
		var encrypted string

		// see selectApiKeyFields
		err := rows.Scan(&encrypted, &apiKey.Team, &apiKey.Created, &apiKey.Expires)
		if err != nil {
			return nil, err
		}

		apiKey.Key, err = db.decrypt(encrypted)
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

// Read all API keys matching the provided team
func (db *Database) ApiKeys(ctx context.Context, team string) (ApiKeys, error) {
	var err error

	query := `SELECT ` + selectApiKeyFields + ` FROM apikey WHERE team = $1 ORDER BY expires DESC;`
	rows, err := db.timedQuery(ctx, query, team)
	if err != nil {
		return nil, err
	}

	return db.scanApiKeyRows(rows)
}

func (db *Database) RotateApiKey(ctx context.Context, team string, key api_v1.Key) error {
	var query string

	encrypted, err := crypto.Encrypt(key, db.encryptionKey)
	if err != nil {
		return fmt.Errorf("encrypt api key: %s", err)
	}

	tx, err := db.conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("unable to start transaction: %s", err)
	}

	query = `UPDATE apikey SET expires = NOW() WHERE expires > NOW() AND team = $1`
	_, err = tx.Exec(ctx, query, team)
	if err != nil {
		return err
	}

	query = `
INSERT INTO apikey (key, team, created, expires)
VALUES ($1, $2, $3, NOW(), NOW()+MAKE_INTERVAL(years := 5));
`
	_, err = tx.Exec(ctx, query, hex.EncodeToString(encrypted), team)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
