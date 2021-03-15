package graphapi

import (
	"context"
	"net/http"

	"github.com/nais/deploy/pkg/hookd/config"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/oauth2/microsoft"
)

type Team struct {
	AzureUUID string
	ID        string // mailNickname
	Title     string // displayName
}

// Valid returns true if the ID fields are non-empty.
func (team Team) Valid() bool {
	return len(team.AzureUUID) > 0 && len(team.ID) > 0
}

type Client interface {
	Team(ctx context.Context, name string) (*Team, error)
	IsErrNotFound(err error) bool
}

type client struct {
	clientID            string
	clientSecret        string
	tenantID            string
	teamMembershipAppID string
}

func (c *client) httpclient(ctx context.Context) *http.Client {
	conf := clientcredentials.Config{
		ClientID:     c.clientID,
		ClientSecret: c.clientSecret,
		Scopes:       []string{"https://graph.microsoft.com/.default"},
		TokenURL:     microsoft.AzureADEndpoint(c.tenantID).TokenURL,
	}

	return conf.Client(ctx)
}

// Team retrieves a NAV team from Azure.
func (c *client) Team(ctx context.Context, name string) (*Team, error) {
	r := &request{
		client: c.httpclient(ctx),
	}

	group, err := r.Group(c.teamMembershipAppID, name)
	if err != nil {
		return nil, err
	}

	return &Team{
		AzureUUID: group.ID,
		ID:        group.MailNickname,
		Title:     group.DisplayName,
	}, nil
}

func (c *client) IsErrNotFound(err error) bool {
	return err == ErrNotFound
}

func NewClient(cfg config.Azure) *client {
	return &client{
		clientID:            cfg.ClientID,
		clientSecret:        cfg.ClientSecret,
		tenantID:            cfg.Tenant,
		teamMembershipAppID: cfg.TeamMembershipAppID,
	}
}
