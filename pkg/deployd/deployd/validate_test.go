package deployd

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func resource(kind, namespace, name string) unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind(kind)
	u.SetName(name)
	if namespace != "" {
		u.SetNamespace(namespace)
	}
	return u
}

func TestValidateTeamNamespaces(t *testing.T) {
	t.Run("resources in own namespace are allowed", func(t *testing.T) {
		resources := []unstructured.Unstructured{
			resource("ConfigMap", "aura", "a"),
			resource("ConfigMap", "aura", "b"),
		}
		if err := validateTeamNamespaces(resources, "aura"); err != nil {
			t.Fatalf("got %v, want nil", err)
		}
	})

	t.Run("foreign namespace is rejected", func(t *testing.T) {
		resources := []unstructured.Unstructured{
			resource("ConfigMap", "aura", "a"),
			resource("ConfigMap", "victim", "b"),
		}
		err := validateTeamNamespaces(resources, "aura")
		if err == nil {
			t.Fatal("got nil, want error")
		}
		want := `resource field .metadata.namespace was "victim", expected "aura"`
		if !strings.HasSuffix(err.Error(), want) {
			t.Fatalf("got %q, want suffix %q", err.Error(), want)
		}
	})

	t.Run("cluster-scoped resource is rejected", func(t *testing.T) {
		resources := []unstructured.Unstructured{
			resource("ClusterRole", "", "cr"),
		}
		err := validateTeamNamespaces(resources, "aura")
		if err == nil {
			t.Fatal("got nil, want error")
		}
		want := `resource field .metadata.namespace is missing; "aura" cannot deploy cluster-scoped resources`
		if !strings.HasSuffix(err.Error(), want) {
			t.Fatalf("got %q, want suffix %q", err.Error(), want)
		}
	})

	t.Run("empty team is rejected", func(t *testing.T) {
		resources := []unstructured.Unstructured{
			resource("ConfigMap", "aura", "a"),
		}
		if err := validateTeamNamespaces(resources, ""); err == nil {
			t.Fatal("got nil, want error")
		}
	})
}
