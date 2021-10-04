package strategy

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func NewDeployStrategy(gvk schema.GroupVersionKind, namespacedResource dynamic.ResourceInterface) DeployStrategy {
	if gvk.Group == "batch" && gvk.Version == "v1" && gvk.Kind == "Job" {
		return recreateStrategy{client: namespacedResource}
	} else {
		return createOrUpdateStrategy{client: namespacedResource}
	}
}

type DeployStrategy interface {
	Deploy(ctx context.Context, resource unstructured.Unstructured) (*unstructured.Unstructured, error)
}

type recreateStrategy struct {
	client dynamic.ResourceInterface
}

type createOrUpdateStrategy struct {
	client dynamic.ResourceInterface
}

func (r recreateStrategy) Deploy(ctx context.Context, resource unstructured.Unstructured) (*unstructured.Unstructured, error) {
	err := r.client.Delete(ctx, resource.GetName(), metav1.DeleteOptions{})
	if !errors.IsNotFound(err) {
		return nil, err
	}
	return r.client.Create(ctx, &resource, metav1.CreateOptions{})
}

func (c createOrUpdateStrategy) Deploy(ctx context.Context, resource unstructured.Unstructured) (*unstructured.Unstructured, error) {
	existing, err := c.client.Get(ctx, resource.GetName(), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		deployed, err := c.client.Create(ctx, &resource, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("get existing resource: %s", err)
		}
		return deployed, nil
	} else if err != nil {
		return nil, fmt.Errorf("get existing resource: %s", err)
	}

	resource.SetResourceVersion(existing.GetResourceVersion())
	return c.client.Update(ctx, &resource, metav1.UpdateOptions{})
}
