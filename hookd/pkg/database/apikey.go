package database

import (
	"time"

	api_v1 "github.com/navikt/deployment/hookd/pkg/api/v1"
)

type ApiKey struct {
	Team    string     `json:"team"`
	GroupId string     `json:"groupId"`
	Key     api_v1.Key `json:"key"`
	Expires time.Time  `json:"expires"`
	Created time.Time  `json:"created"`
}

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

const selectApiKeyFields = `key, team, team_azure_id, created, expires`
