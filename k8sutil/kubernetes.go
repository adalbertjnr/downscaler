package k8sutil

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/adalbertjnr/downscaler/helpers"
	"github.com/adalbertjnr/downscaler/shared"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type KubernetesHelper interface {
	GetNamespaces(ctx context.Context) []string
	GetDeployments(ctx context.Context, namespace string) *v1.DeploymentList
	GetDownscalerData(ctx context.Context, gv schema.GroupVersionResource) (*shared.DownscalerPolicy, error)
	DownscaleDeployments(ctx context.Context, namespace string, deployment *v1.Deployment)
	GetWatcherByDownscalerCRD(ctx context.Context, name, namespace string) (watch.Interface, error)
}

type KubernetesHelperImpl struct {
	K8sClient     *kubernetes.Clientset
	DynamicClient *dynamic.DynamicClient
}

func NewKubernetesHelper(
	client *kubernetes.Clientset,
	dynamicClient *dynamic.DynamicClient,
) *KubernetesHelperImpl {
	return &KubernetesHelperImpl{
		K8sClient:     client,
		DynamicClient: dynamicClient,
	}
}

func (kuberneterActor KubernetesHelperImpl) GetDownscalerData(
	ctx context.Context,
	gv schema.GroupVersionResource,
) (*shared.DownscalerPolicy, error) {
	list, err := kuberneterActor.DynamicClient.Resource(gv).Namespace("").List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", "downscaler").String(),
	})
	if err != nil {
		return nil, err
	}

	data := &shared.DownscalerPolicy{}
	obj := list.Items[0]
	if err := helpers.UnmarshalDataPolicy(obj, data); err != nil {
		slog.Error("unmarshaling", "error", err)
		return nil, err
	}

	return data, nil
}

func (kuberneterActor KubernetesHelperImpl) GetNamespaces(ctx context.Context) []string {
	var namespacesNames []string
	namespaces, err := kuberneterActor.K8sClient.CoreV1().
		Namespaces().
		List(ctx, metav1.ListOptions{})

	if err != nil {
		slog.Error("listing namespaces", "error", err)
		return nil
	}

	for _, namespace := range namespaces.Items {
		namespacesNames = append(namespacesNames, namespace.Name)
	}

	return namespacesNames
}

func (kuberneterActor KubernetesHelperImpl) GetDeployments(ctx context.Context, namespace string) *v1.DeploymentList {
	deployments, err := kuberneterActor.K8sClient.AppsV1().
		Deployments(namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Error("get deployments error", "namespace", namespace, "error", err)
		return nil
	}
	return deployments
}

func (kuberneterActor KubernetesHelperImpl) DownscaleDeployments(ctx context.Context, namespace string, deployment *v1.Deployment) {
	desiredReplicas := int32(0)
	currentReplicas := *deployment.Spec.Replicas

	deployment.Spec.Replicas = &desiredReplicas
	_, err := kuberneterActor.K8sClient.AppsV1().
		Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		slog.Error("downscaling error", "deployment", deployment.Name, "error", err.Error())
		return
	}
	slog.Info("downscaling",
		"name", deployment.Name,
		"namespace", deployment.Name,
		"old state replicas", currentReplicas,
		"current state replicas", desiredReplicas,
		"status", "success",
	)
}

func (kubernetesActor KubernetesHelperImpl) GetWatcherByDownscalerCRD(ctx context.Context, name, namespace string) (watch.Interface, error) {
	timeout := int64(3600)
	watcher, err := kubernetesActor.DynamicClient.Resource(schema.GroupVersionResource{
		Group:    shared.Group,
		Version:  shared.Version,
		Resource: shared.Resource,
	}).Namespace("").Watch(ctx, metav1.ListOptions{
		FieldSelector:  fields.OneTermEqualSelector("metadata.name", name).String(),
		TimeoutSeconds: &timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create the watcher. downscaler name %s. err: %v", name, err)
	}
	slog.Info("watcher",
		"group", shared.Group,
		"version", shared.Version,
		"resource", shared.Resource,
		"status", "created",
	)
	return watcher, nil
}

func InitDownscalingProcess(ctx context.Context,
	k8sClient KubernetesHelper,
	namespaces []string,
	ignoredNamespaces map[string]struct{},
	criteriaList map[string]struct{},
) {

	for _, namespace := range namespaces {
		if _, exists := ignoredNamespaces[namespace]; exists {
			slog.Info("downscaling message",
				"ignoring namespace", namespace,
				"reason", "already ignored from the config",
			)
			continue
		}

		if namespace == shared.AnyOther {
			go triggerAnyOther(ctx, k8sClient, criteriaList, ignoredNamespaces)
			continue
		}

		deployments := k8sClient.GetDeployments(ctx, namespace)
		for _, deployment := range deployments.Items {
			k8sClient.DownscaleDeployments(ctx, namespace, &deployment)
		}
	}

}

func triggerAnyOther(ctx context.Context,
	k8sClient KubernetesHelper,
	scheduledNamespaces map[string]struct{},
	ignoredNamespaces map[string]struct{},
) {

	clusterNamespaces := k8sClient.GetNamespaces(ctx)
	for _, clusterNamespace := range clusterNamespaces {

		if _, exists := ignoredNamespaces[clusterNamespace]; exists {
			slog.Info("downscaling message",
				"ignoring namespace", clusterNamespace,
				"reason", "already ignored from the config",
			)
			continue
		}

		if _, scheduled := scheduledNamespaces[clusterNamespace]; scheduled {
			slog.Info("downscaling message",
				"ignoring namespace", clusterNamespace,
				"reason", "already scheduled by another routine",
			)
			continue
		}

		deployments := k8sClient.GetDeployments(ctx, clusterNamespace)
		for _, deployment := range deployments.Items {
			k8sClient.DownscaleDeployments(ctx, clusterNamespace, &deployment)
		}
	}
}
