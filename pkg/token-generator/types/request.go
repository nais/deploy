package types

import (
	"fmt"
)

type Source string

type Sink string

// Request payload submitted when making a token request.
type Request struct {
	Repository string   `json:"repository"`
	Sources    []Source `json:"sources"`
	Sinks      []Sink   `json:"sinks"`
}

func (r *Request) Validate() error {
	if len(r.Sources) > 0 && len(r.Sinks) > 0 {
		return nil
	}
	return fmt.Errorf("token requests must specify at least one source and at least one destination")
}
