package deployclient

import (
	"encoding/json"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func InjectAnnotations(resource json.RawMessage, annotations map[string]string) (json.RawMessage, error) {
	decoded := make(map[string]json.RawMessage)
	err := json.Unmarshal(resource, &decoded)
	if err != nil {
		return nil, err
	}

	meta := &v1.ObjectMeta{}
	err = json.Unmarshal(decoded["metadata"], meta)
	if err != nil {
		return nil, err
	}

	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		meta.Annotations[k] = v
	}
	encoded, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	decoded["metadata"] = encoded
	return json.Marshal(decoded)
}
