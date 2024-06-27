package naisapi

import (
	"context"
	"fmt"

	"github.com/nais/deploy/pkg/naisapi/protoapi"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	client protoapi.TeamsClient
}

func NewClient(target string, insecureConnection bool) (*Client, error) {
	opts := []grpc.DialOption{}
	if insecureConnection {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	gclient, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to nais-api: %w", err)
	}

	return &Client{
		client: protoapi.NewTeamsClient(gclient),
	}, nil
}

func (c *Client) IsAuthorized(ctx context.Context, repo, team string) bool {
	resp, err := c.client.IsRepositoryAuthorized(ctx, &protoapi.IsRepositoryAuthorizedRequest{
		TeamSlug:   team,
		Repository: repo,
	})
	if err != nil {
		log.WithError(err).Error("checking repo authorization in teams")
		return false
	}

	return resp.IsAuthorized
}
