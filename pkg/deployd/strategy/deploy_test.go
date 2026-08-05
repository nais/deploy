package strategy

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

var (
	applicationGVK = schema.GroupVersionKind{Group: "nais.io", Version: "v1alpha1", Kind: "Application"}
	applicationGVR = schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}
	naisjobGVK     = schema.GroupVersionKind{Group: "nais.io", Version: "v1", Kind: "Naisjob"}
	naisjobGVR     = schema.GroupVersionResource{Group: "nais.io", Version: "v1", Resource: "naisjobs"}
)

// Redeploying an unchanged spec must still trigger Naiserator, which only synchronizes
// when its stored hash differs from the hash it computes.
func TestDeployInvalidatesSynchronizationHash(t *testing.T) {
	tests := []struct {
		name string
		gvk  schema.GroupVersionKind
		gvr  schema.GroupVersionResource
	}{
		{name: "Application", gvk: applicationGVK, gvr: applicationGVR},
		{name: "Naisjob", gvk: naisjobGVK, gvr: naisjobGVR},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := testResource(tt.gvk)
			require.NoError(t, unstructured.SetNestedField(existing.Object, "current-hash", "status", "synchronizationHash"))
			desired := existing.DeepCopy()

			client := fake.NewSimpleDynamicClient(runtime.NewScheme(), &existing)
			resourceClient := client.Resource(tt.gvr).Namespace(existing.GetNamespace())

			deployed, err := NewDeployStrategy(resourceClient).Deploy(t.Context(), *desired, noop.Span{})
			require.NoError(t, err)
			require.NotNil(t, deployed)

			// The spec must be written before the hash is cleared, so that the
			// resynchronization picks up this deployment rather than the previous one.
			require.Equal(t, []string{"get", "update", "patch"}, verbs(client.Actions()))

			patch := lastPatch(t, client.Actions())
			require.Equal(t, "status", patch.GetSubresource())
			require.Equal(t, types.MergePatchType, patch.GetPatchType())
			require.JSONEq(t, `{"status":{"synchronizationHash":null}}`, string(patch.GetPatch()))
		})
	}
}

func TestDeployDoesNotInvalidateSynchronizationHash(t *testing.T) {
	tests := []struct {
		name     string
		gvk      schema.GroupVersionKind
		gvr      schema.GroupVersionResource
		existing bool
	}{
		{
			name: "new Application is synchronized on creation",
			gvk:  applicationGVK,
			gvr:  applicationGVR,
		},
		{
			name:     "ConfigMap has no synchronization hash",
			gvk:      schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
			gvr:      schema.GroupVersionResource{Version: "v1", Resource: "configmaps"},
			existing: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := testResource(tt.gvk)
			client := fake.NewSimpleDynamicClient(runtime.NewScheme())
			resourceClient := client.Resource(tt.gvr).Namespace(resource.GetNamespace())
			if tt.existing {
				_, err := resourceClient.Create(t.Context(), &resource, metav1.CreateOptions{})
				require.NoError(t, err)
				client.ClearActions()
			}

			_, err := NewDeployStrategy(resourceClient).Deploy(t.Context(), resource, noop.Span{})
			require.NoError(t, err)
			require.NotContains(t, verbs(client.Actions()), "patch")
		})
	}
}

// A resource whose hash was not cleared silently stops reconciling, so the deployment
// must fail loudly rather than wait for a rollout that never happens.
func TestDeployFailsWhenSynchronizationHashCannotBeInvalidated(t *testing.T) {
	existing := testResource(applicationGVK)
	desired := existing.DeepCopy()

	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), &existing)
	client.PrependReactor("patch", "applications", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.NewForbidden(applicationGVR.GroupResource(), existing.GetName(), nil)
	})
	resourceClient := client.Resource(applicationGVR).Namespace(existing.GetNamespace())

	_, err := NewDeployStrategy(resourceClient).Deploy(t.Context(), *desired, noop.Span{})
	require.ErrorContains(t, err, "invalidating synchronization hash")
	require.True(t, errors.IsForbidden(err))
}

func verbs(actions []k8stesting.Action) []string {
	found := make([]string, 0, len(actions))
	for _, action := range actions {
		found = append(found, action.GetVerb())
	}
	return found
}

func lastPatch(t *testing.T, actions []k8stesting.Action) k8stesting.PatchAction {
	t.Helper()
	for i := len(actions) - 1; i >= 0; i-- {
		if patch, ok := actions[i].(k8stesting.PatchAction); ok {
			return patch
		}
	}
	t.Fatal("expected a patch action")
	return nil
}

func testResource(gvk schema.GroupVersionKind) unstructured.Unstructured {
	resource := unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name":      "test-resource",
			"namespace": "test-namespace",
		},
	}}
	resource.SetGroupVersionKind(gvk)
	return resource
}
