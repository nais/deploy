package strategy

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

func NewDeployStrategy(namespacedResource dynamic.ResourceInterface) DeployStrategy {
	return createOrUpdateStrategy{client: namespacedResource}
}

type DeployStrategy interface {
	Deploy(ctx context.Context, resource unstructured.Unstructured) (*unstructured.Unstructured, error)
}

type createOrUpdateStrategy struct {
	client dynamic.ResourceInterface
}

func (c createOrUpdateStrategy) Deploy(ctx context.Context, resource unstructured.Unstructured) (*unstructured.Unstructured, error) {
	existing, err := c.client.Get(ctx, resource.GetName(), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		deployed, err := c.client.Create(ctx, &resource, metav1.CreateOptions{
			FieldValidation: metav1.FieldValidationStrict,
		})
		if err != nil {
			return nil, fmt.Errorf("creating resource: %w", transformStrictDecodingError(resource, err))
		}
		return deployed, nil
	} else if err != nil {
		return nil, fmt.Errorf("get existing resource: %w", err)
	}

	resource.SetResourceVersion(existing.GetResourceVersion())
	updated, err := c.client.Update(ctx, &resource, metav1.UpdateOptions{
		FieldValidation: metav1.FieldValidationStrict,
	})
	if err != nil {
		return nil, fmt.Errorf("updating resource: %w", transformStrictDecodingError(resource, err))
	}

	return updated, nil
}

func transformStrictDecodingError(resource unstructured.Unstructured, err error) error {
	msg := err.Error()

	// Kubernetes doesn't expose any error types, so we have to rely on the error message for now
	const strictDecodingError = "strict decoding error:"

	// we only transform strict decoding errors
	if !strings.Contains(msg, strictDecodingError) {
		return err
	}

	// we trim the default error message as it is too verbose, e.g:
	// > Application in version "v1alpha1" cannot be handled as a Application: strict decoding error: unknown field "spec.nestedField", ...
	if strings.Contains(msg, strictDecodingError) {
		parts := strings.SplitAfterN(msg, strictDecodingError, 2)
		if len(parts) > 1 {
			msg = parts[1]
		}
	}

	docs := map[string]string{
		"aiven.io/v1alpha1, Kind=OpenSearch":            "https://doc.nais.io/persistence/open-search/#creating-a-opensearch-instance",
		"aiven.io/v1alpha1, Kind=Redis":                 "https://doc.nais.io/persistence/redis/#creating-a-redis-instance-explicitly",
		"aiven.io/v1alpha1, Kind=ServiceIntegration":    "https://doc.nais.io/persistence/open-search/#serviceintegration",
		"kafka.nais.io/v1, Kind=Topic":                  "https://doc.nais.io/persistence/kafka/topic/",
		"krakend.nais.io/v1, Kind=ApiEndpoints":         "https://doc.nais.io/security/apigateway/",
		"monitoring.coreos.com/v1, Kind=PrometheusRule": "https://doc.nais.io/observability/alerts/#kubernetes-resources",
		"nais.io/v1alpha1, Kind=Application":            "https://doc.nais.io/nais-application/application/",
		"nais.io/v1, Kind=Naisjob":                      "https://doc.nais.io/naisjob/reference/",
		"unleash.nais.io/v1, Kind=ApiToken":             "https://doc.nais.io/addons/unleash-next/#creating-a-new-api-token",
	}

	s := &strings.Builder{}
	s.WriteString(strictDecodingError)

	// multiple errors are joined as a comma separated string; split them up again
	errs := strings.Split(msg, ",")
	for _, e := range errs {
		s.WriteString("\n| ⚠️ ")
		s.WriteString(strings.TrimSpace(e))
	}

	s.WriteString("\n| The field")
	if len(errs) > 1 {
		s.WriteString("s")
	}
	s.WriteString(" might be misspelled, incorrectly indented, or unsupported.")

	s.WriteString("\n| Please verify your resource against the reference documentation")
	if u, ok := docs[resource.GroupVersionKind().String()]; ok {
		s.WriteString(" at " + u)
	}

	return fmt.Errorf(s.String())
}
