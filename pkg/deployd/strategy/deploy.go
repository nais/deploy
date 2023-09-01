package strategy

import (
	"context"
	"fmt"

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
			FieldValidation: metav1.FieldValidationWarn,
		})
		if err != nil {
			return nil, fmt.Errorf("creating resource: %s", err)
		}
		return deployed, nil
	} else if err != nil {
		return nil, fmt.Errorf("get existing resource: %s", err)
	}

	resource.SetResourceVersion(existing.GetResourceVersion())
	updated, err := c.client.Update(ctx, &resource, metav1.UpdateOptions{
		FieldValidation: metav1.FieldValidationWarn,
	})
	if err != nil {
		return nil, fmt.Errorf("updating resource: %s", err)
	}
	return updated, nil
}
