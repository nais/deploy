package naisapi

import (
	"context"

	"github.com/nais/api/pkg/apiclient"
	"github.com/nais/api/pkg/protoapi"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	client protoapi.TeamsClient
}

func New(target string, insecureConnection bool) (*Client, error) {
	opts := []grpc.DialOption{}
	if insecureConnection {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	client, err := apiclient.New(target, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{
		client: client.Teams(),
	}, nil
}

func (c *Client) IsAuthorized(ctx context.Context, repo, team string) bool {
	resp, err := c.client.IsRepositoryAuthorized(ctx, &protoapi.IsRepositoryAuthorizedRequest{
		TeamSlug:      team,
		Repository:    repo,
		Authorization: protoapi.RepositoryAuthorization_DEPLOY,
	})
	if err != nil {
		log.WithError(err).Error("checking repo authorization in teams")
		return false
	}

	return resp.IsAuthorized
}
