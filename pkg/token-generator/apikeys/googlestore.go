package apikeys

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

var _ Source = &googleStore{}

var objectName = "apiKeys"

type googleStore struct {
	client      *storage.Client
	memoryStore *memoryStore
	bucketName  string
}

func NewGoogleCloudStore(credentialsFile, bucketName string) (Source, error) {
	client, err := storage.NewClient(context.Background(), option.WithCredentialsFile(credentialsFile))
	if err != nil {
		return nil, err
	}

	store := &googleStore{
		client:      client,
		bucketName:  bucketName,
		memoryStore: NewMemoryStore(),
	}

	err = store.load()
	if err != nil {
		return nil, fmt.Errorf("loading API keys: %s", err)
	}

	return store, nil
}

func (g *googleStore) object() *storage.ObjectHandle {
	return g.client.Bucket(g.bucketName).Object(objectName)
}

func (g *googleStore) load() error {
	r, err := g.object().NewReader(context.Background())
	if err != nil {
		// Non-existing object. Continue if we can create it.
		if err == storage.ErrObjectNotExist {
			return g.save()
		}
		return err
	}

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &g.memoryStore.keys)
	if err != nil {
		return err
	}

	return nil
}

func (g *googleStore) save() error {
	w := g.object().NewWriter(context.Background())
	payload, err := json.Marshal(g.memoryStore.keys)
	if err != nil {
		return fmt.Errorf("unable to marshal api key store: %s", err)
	}

	written, err := w.Write(payload)
	if err != nil {
		return err
	}
	if written != len(payload) {
		return fmt.Errorf("cannot write to %s/%s", g.bucketName, objectName)
	}

	return w.Close()
}

func (g *googleStore) Write(team, key string) error {
	err := g.memoryStore.Write(team, key) // always returns nil
	if err != nil {
		return nil
	}

	// make a backup of the underlying store
	backup := make(map[string]string)
	for key, value := range g.memoryStore.keys {
		backup[key] = value
	}

	err = g.save()

	// revert data if remote persist fails
	if err != nil {
		g.memoryStore.keys = backup
	}

	return err
}

func (g *googleStore) Validate(team, key string) error {
	return g.memoryStore.Validate(team, key)
}
