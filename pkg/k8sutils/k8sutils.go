package k8sutils

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Identifier struct {
	schema.GroupVersionKind
	Namespace string
	Name      string
	Cluster   string
}

func ResourceIdentifier(resource unstructured.Unstructured) Identifier {
	return Identifier{
		GroupVersionKind: resource.GroupVersionKind(),
		Namespace:        resource.GetNamespace(),
		Name:             resource.GetName(),
	}
}

func ResourcesFromJSON(json []json.RawMessage) ([]unstructured.Unstructured, error) {
	resources := make([]unstructured.Unstructured, len(json))
	for i := range resources {
		err := resources[i].UnmarshalJSON(json[i])
		if err != nil {
			return nil, fmt.Errorf("resource %d: decoding payload: %s", i+1, err)
		}
	}
	return resources, nil
}
