package client

import (
	"context"
	"fmt"

	downscalergov1alpha1 "github.com/adalbertjnr/downscaler/api/v1alpha1"
	"github.com/adalbertjnr/downscaler/internal/utils"
	appsv1 "k8s.io/api/apps/v1"
	v2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type APIClient struct {
	client.Client

	ctx context.Context
}

func NewAPIClient(c client.Client) *APIClient {
	return &APIClient{
		ctx:    context.Background(),
		Client: c,
	}
}

func (c *APIClient) GetNamespaces() (*v1.NamespaceList, error) {
	var namespaces v1.NamespaceList

	if err := c.Client.List(c.ctx, &namespaces); err != nil {
		return nil, err
	}

	return &namespaces, nil
}

func (c *APIClient) Patch(replicas int, object any) error {
	patchOpts := client.Merge
	replicaCount := int32(replicas)

	switch value := object.(type) {
	case *appsv1.Deployment:
		value.Spec.Replicas = &replicaCount
		return c.Client.Patch(c.ctx, value, patchOpts)
	case *appsv1.StatefulSet:
		value.Spec.Replicas = &replicaCount
		return c.Client.Patch(c.ctx, value, patchOpts)
	case *v2.HorizontalPodAutoscaler:
		value.Spec.MinReplicas = &replicaCount
		return c.Client.Patch(c.ctx, value, patchOpts)
	default:
		return fmt.Errorf("resource type for patching found")
	}
}

func (c *APIClient) Get(namespace string, resource any, name ...string) error {
	listOpts := &client.ListOptions{Namespace: namespace}

	switch value := resource.(type) {
	case *appsv1.Deployment:
		return c.Client.Get(c.ctx, types.NamespacedName{Name: name[0], Namespace: namespace}, value)
	case *appsv1.StatefulSet:
		return c.Client.Get(c.ctx, types.NamespacedName{Name: name[0], Namespace: namespace}, value)
	case *appsv1.DeploymentList:
		return c.Client.List(c.ctx, value, listOpts)
	case *appsv1.StatefulSetList:
		return c.Client.List(c.ctx, value, listOpts)
	case *v2.HorizontalPodAutoscalerList:
		return c.Client.List(c.ctx, value, listOpts)
	default:
		return fmt.Errorf("the resource type was not found for get")
	}
}

func (c *APIClient) GetDownscaler(downscalerObject downscalergov1alpha1.Downscaler) (downscaler downscalergov1alpha1.Downscaler, err error) {
	namespace, err := utils.GetNamespace()
	if err != nil {
		namespace = "kubetime-scaler"
	}

	if err := c.Client.Get(context.Background(), types.NamespacedName{
		Name:      downscalerObject.Name,
		Namespace: namespace,
	}, &downscaler); err != nil {
		return downscaler, err
	}

	return downscaler, nil
}
