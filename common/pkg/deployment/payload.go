package deployment

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/golang/protobuf/jsonpb"
)

// We must use the jsonpb package to unmarshal data into a []*structpb.Struct data structure.
// The jsonpb.Unmarshal function must unmarshal into a type that satisfies the Proto interface.
// This function wraps the provided raw data into a higher level data structure (Kubernetes)
// and returns that object instead.
func KubernetesFromJSONResources(resources json.RawMessage) (*Kubernetes, error) {
	type wrapped struct {
		Resources json.RawMessage `json:"resources"`
	}

	w := &wrapped{
		Resources: resources,
	}
	sr, err := json.Marshal(w)
	if err != nil {
		return nil, fmt.Errorf("unable to wrap kubernetes resources: %s", err)
	}
	kube := &Kubernetes{}
	reader := bytes.NewReader(sr)

	if err := jsonpb.Unmarshal(reader, kube); err != nil {
		return nil, fmt.Errorf("unable to unmarshal kubernetes resources: %s", err)
	}

	return kube, nil
}

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
