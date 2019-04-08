package deployment

import (
	"bytes"
	"encoding/json"

	"github.com/golang/protobuf/jsonpb"
)

func PayloadFromJSON(data []byte) (*Payload, error) {
	r := bytes.NewReader(data)
	p := &Payload{}
	err := jsonpb.Unmarshal(r, p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (m *Payload) JSONResources() ([]json.RawMessage, error) {
	resources := m.GetKubernetes().GetResources()
	msgs := make([]json.RawMessage, len(resources))
	mar := jsonpb.Marshaler{}

	for i, r := range resources {
		s, err := mar.MarshalToString(r)
		if err != nil {
			return nil, err
		}
		msgs[i] = []byte(s)
	}

	return msgs, nil
}
