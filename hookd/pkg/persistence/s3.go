package persistence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/minio/minio-go"
	"github.com/navikt/deployment/hookd/pkg/config"
	log "github.com/sirupsen/logrus"
	"io"
)

const (
	payloadMaxSize = 65536
)

type TeamRepositoryStorage interface {
	Read(repository string) ([]string, error)
	Write(repository string, teams []string) error
}

type s3storage struct {
	config config.S3
	client *minio.Client
}

func NewS3StorageBackend(cfg config.S3) (TeamRepositoryStorage, error) {
	client, err := minio.New(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey, cfg.UseTLS)
	if err != nil {
		return nil, fmt.Errorf("while setting up S3 client: %s", err)
	}
	return &s3storage{
		client: client,
		config: cfg,
	}, nil
}

func (s *s3storage) ensureBucket() error {
	exists, err := s.client.BucketExists(s.config.BucketName)
	if err != nil {
		return fmt.Errorf("unable to query S3 bucket status: %s", err)
	}
	if exists {
		return nil
	}
	err = s.client.MakeBucket(s.config.BucketName, s.config.BucketLocation)
	if err == nil {
		log.Tracef("S3: created bucket '%s' at location '%s'", s.config.BucketName, s.config.BucketLocation)
	}
	return err
}

func (s *s3storage) Read(repository string) ([]string, error) {
	if err := s.ensureBucket(); err != nil {
		return nil, err
	}
	obj, err := s.client.GetObject(s.config.BucketName, repository, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("while locating s3 object: %s", err)
	}
	stat, err := obj.Stat()
	if err != nil {
		return nil, fmt.Errorf("while querying s3 object stats: %s", err)
	}

	payload := make([]byte, 0, stat.Size)
	w := bytes.NewBuffer(payload)
	n, err := io.CopyN(w, obj, stat.Size)
	if err != nil {
		return nil, fmt.Errorf("while reading from s3 bucket: %s", err)
	}
	log.Tracef("S3: read %d bytes from %s/%s", n, s.config.BucketName, repository)
	log.Tracef("S3: payload was: %s", string(w.Bytes()))

	teams := make([]string, 0)
	err = json.Unmarshal(w.Bytes(), &teams)
	if err != nil {
		return nil, fmt.Errorf("while unmarshalling s3 data: %s", err)
	}

	return teams, nil
}

func (s *s3storage) Write(repository string, teams []string) error {
	if err := s.ensureBucket(); err != nil {
		return err
	}
	payload, err := json.Marshal(teams)
	if err != nil {
		return fmt.Errorf("while marshalling s3 data: %s", err)
	}
	r := bytes.NewReader(payload)
	n, err := s.client.PutObject(s.config.BucketName, repository, r, r.Size(), minio.PutObjectOptions{})
	if err == nil {
		log.Tracef("S3: wrote %d bytes to %s/%s", n, s.config.BucketName, repository)
		log.Tracef("S3: payload was: %s", string(payload))
	}
	return err
}
