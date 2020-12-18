package k8sutils

import (
	"encoding/json"
	"fmt"

	"github.com/navikt/deployment/pkg/pb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Identifier struct {
	schema.GroupVersionKind
	Namespace string
	Name      string
}

func (id Identifier) String() string {
	if len(id.Namespace) > 0 {
		return fmt.Sprintf("%s, Namespace=%s, Name=%s", id.GroupVersionKind.String(), id.Namespace, id.Name)
	}
	return fmt.Sprintf("%s, Name=%s", id.GroupVersionKind.String(), id.Name)
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

func ResourcesFromDeploymentRequest(request *pb.DeploymentRequest) ([]unstructured.Unstructured, error) {
	js, err := request.GetPayloadSpec().JSONResources()
	if err != nil {
		return nil, err
	}
	return ResourcesFromJSON(js)
}

func Identifiers(resources []unstructured.Unstructured) []Identifier {
	identifiers := make([]Identifier, len(resources))
	for i := range resources {
		identifiers[i] = ResourceIdentifier(resources[i])
	}
	return identifiers
}
