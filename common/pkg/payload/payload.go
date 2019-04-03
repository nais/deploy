package payload

import (
	"encoding/json"
)

type Payload struct {
	Version    [3]int
	Team       string
	Kubernetes struct {
		Resources []json.RawMessage
	}
}
