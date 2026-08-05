package strategy

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

func NewDeployStrategy(namespacedResource dynamic.ResourceInterface) DeployStrategy {
	return createOrUpdateStrategy{client: namespacedResource}
}

type DeployStrategy interface {
	Deploy(ctx context.Context, resource unstructured.Unstructured, trace trace.Span) (*unstructured.Unstructured, error)
}

type createOrUpdateStrategy struct {
	client dynamic.ResourceInterface
}

func (c createOrUpdateStrategy) Deploy(ctx context.Context, resource unstructured.Unstructured, trace trace.Span) (*unstructured.Unstructured, error) {
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

	if shouldInvalidateSynchronizationHash(resource) {
		err = c.invalidateSynchronizationHash(ctx, resource.GetName())
		if err != nil {
			return nil, fmt.Errorf("invalidating synchronization hash: %w", err)
		}
		trace.AddEvent("Forced resynchronization of Nais resource")
	}

	return updated, nil
}

func shouldInvalidateSynchronizationHash(resource unstructured.Unstructured) bool {
	gvk := resource.GroupVersionKind()
	return gvk.Group == "nais.io" && (gvk.Kind == "Application" || gvk.Kind == "Naisjob")
}

// invalidateSynchronizationHash clears the hash Naiserator compares against to decide
// whether a resource needs synchronization. Without this, redeploying an unchanged spec
// is a no-op, and the deployment waits for a rollout event that never arrives.
//
// The resource spec is written before this call, so the resynchronization picks up the
// spec and correlation ID from this deployment. Only the hash field is patched, leaving
// the rest of the operator-owned status untouched.
func (c createOrUpdateStrategy) invalidateSynchronizationHash(ctx context.Context, name string) error {
	_, err := c.client.Patch(ctx, name, types.MergePatchType,
		[]byte(`{"status":{"synchronizationHash":null}}`), metav1.PatchOptions{}, "status")
	return err
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

	docs := map[string]string{ // #nosec G101 -- documentation URLs, not credentials
		"aiven.io/v1alpha1, Kind=OpenSearch":            "https://doc.nais.io/persistence/opensearch/how-to/create/",
		"aiven.io/v1alpha1, Kind=Redis":                 "https://doc.nais.io/persistence/redis/",
		"aiven.io/v1alpha1, Kind=ServiceIntegration":    "https://doc.nais.io/persistence/opensearch/how-to/create/#serviceintegration",
		"kafka.nais.io/v1, Kind=Topic":                  "https://doc.nais.io/persistence/kafka/how-to/create/",
		"monitoring.coreos.com/v1, Kind=PrometheusRule": "https://doc.nais.io/observability/alerting/reference/prometheusrule/",
		"nais.io/v1alpha1, Kind=Application":            "https://doc.nais.io/workloads/application/reference/application-spec/",
		"nais.io/v1, Kind=Naisjob":                      "https://doc.nais.io/workloads/job/reference/naisjob-spec/",
		"unleash.nais.io/v1, Kind=ApiToken":             "https://doc.nais.io/services/feature-toggling/?h=#creating-a-new-api-token",
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
	s.WriteString(" might be misspelled, incorrectly indented, or unsupported. Fields are case sensitive.")

	s.WriteString("\n| Please verify your resource against the reference documentation")
	if u, ok := docs[resource.GroupVersionKind().String()]; ok {
		s.WriteString(" at " + u)
	}

	return fmt.Errorf("%s", s.String())
}
