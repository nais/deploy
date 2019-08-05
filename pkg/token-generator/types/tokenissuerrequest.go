package types

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jjeffery/stringset"
)

type Source string

type Sink string

// Request payload submitted when making a token request.
type TokenIssuerRequest struct {
	Repository string        `json:"repository"`
	Sources    stringset.Set `json:"sources"`
	Sinks      stringset.Set `json:"sinks"`

	// Metadata
	ID      string          `json:",omitempty"`
	Context context.Context `json:",omitempty"`
}

func (r *TokenIssuerRequest) Validate() error {
	if len(r.Sources) > 0 && len(r.Sinks) > 0 {
		return nil
	}
	return fmt.Errorf("token requests must specify at least one source and at least one sink")
}

func (r *TokenIssuerRequest) Bind(request *http.Request) error {
	return r.Validate()
}
