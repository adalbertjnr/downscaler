package k8s

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/adalbertjnr/downscaler/helpers"
	"github.com/adalbertjnr/downscaler/shared"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type Kubernetes interface {
	GetNamespaces(ctx context.Context) []string
	GetDeployments(ctx context.Context, namespace string) *v1.DeploymentList
	GetDownscalerData(ctx context.Context, gv schema.GroupVersionResource) (*shared.DownscalerPolicy, error)
	DownscaleDeployments(ctx context.Context, namespace string, deployment *v1.Deployment)
	GetWatcherByDownscalerCRD(ctx context.Context, name, namespace string) (watch.Interface, error)
	StartDownscaling(ctx context.Context, namespaces []string, is shared.NotUsableNamespacesDuringScheduling) map[string][]string
	ListConfigMap(ctx context.Context, name, namespace string) *corev1.ConfigMap
	PatchConfigMap(ctx context.Context, name, namespace string, patch []byte)
	CreateConfigMap(ctx context.Context, name, namespace string) error
}

type KubernetesImpl struct {
	K8sClient     *kubernetes.Clientset
	DynamicClient *dynamic.DynamicClient
}

func NewKubernetes(client *kubernetes.Clientset, dynamicClient *dynamic.DynamicClient) *KubernetesImpl {
	return &KubernetesImpl{
		K8sClient:     client,
		DynamicClient: dynamicClient,
	}
}

func (k KubernetesImpl) CreateConfigMap(ctx context.Context, name, namespace string) error {
	create := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{},
	}

	_, err := k.K8sClient.CoreV1().ConfigMaps(namespace).Create(ctx, create, metav1.CreateOptions{})
	if err != nil {
		slog.Error("not able to create configmap", "name", name, "namespace", namespace, "reason", err)
		return err
	}

	slog.Info("configmap created", "name", name, "namespace", namespace)
	return nil
}

func (k KubernetesImpl) ListConfigMap(ctx context.Context, name, namespace string) *corev1.ConfigMap {
	cm, err := k.K8sClient.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name).String(),
	},
	)

	if err != nil {
		slog.Error("configmap error",
			"not possible to get configmap", name,
			"namespace", namespace,
		)
	}

	if len(cm.Items) > 0 {
		return &cm.Items[0]
	}

	return nil
}

func (k KubernetesImpl) PatchConfigMap(ctx context.Context, name, namespace string, patch []byte) {
	_, err := k.K8sClient.CoreV1().ConfigMaps(namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		slog.Error("configmap error",
			"not possible to update configmap", name,
			"namespace", namespace,
			"error", err,
		)
	}

	slog.Info("configmap updated", "name", name, "namespace", namespace)
}

func (k KubernetesImpl) GetDownscalerData(ctx context.Context, gv schema.GroupVersionResource) (*shared.DownscalerPolicy, error) {
	list, err := k.DynamicClient.Resource(gv).Namespace("").List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", "downscaler").String(),
	})
	if err != nil {
		return nil, err
	}

	data := &shared.DownscalerPolicy{}
	obj := &list.Items[0]
	if err := helpers.UnmarshalDataPolicy(obj, data); err != nil {
		slog.Error("unmarshaling", "error", err)
		return nil, err
	}

	return data, nil
}

func (k KubernetesImpl) GetNamespaces(ctx context.Context) []string {
	namespaces, err := k.K8sClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})

	if err != nil {
		slog.Error("listing namespaces", "error", err)
		return nil
	}

	namespacesNames := []string{}
	for _, namespace := range namespaces.Items {
		namespacesNames = append(namespacesNames, namespace.Name)
	}

	return namespacesNames
}

func (k KubernetesImpl) GetDeployments(ctx context.Context, namespace string) *v1.DeploymentList {
	deployments, err := k.K8sClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Error("get deployments error", "namespace", namespace, "error", err)
		return nil
	}
	return deployments
}

func (k KubernetesImpl) DownscaleDeployments(ctx context.Context, namespace string, deployment *v1.Deployment) {
	desiredReplicas := int32(0)
	currentReplicas := *deployment.Spec.Replicas

	deployment.Spec.Replicas = &desiredReplicas
	_, err := k.K8sClient.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		slog.Error("downscaling error",
			"deployment", deployment.Name,
			"namespace", namespace,
			"error", err)
		return
	}
	slog.Info("downscaling message",
		"name", deployment.Name,
		"namespace", namespace,
		"old state replicas", currentReplicas,
		"current state replicas", desiredReplicas,
		"status", "success",
	)
}

func (k KubernetesImpl) GetWatcherByDownscalerCRD(ctx context.Context, name, namespace string) (watch.Interface, error) {
	timeout := int64(3600)
	watcher, err := k.DynamicClient.Resource(schema.GroupVersionResource{
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

func (k KubernetesImpl) StartDownscaling(ctx context.Context, namespaces []string, evicted shared.NotUsableNamespacesDuringScheduling,
) map[string][]string {
	deploymentStateByNamespace := make(map[string][]string)
	for _, namespace := range namespaces {
		if isNamespaceIgnored(namespace, evicted) {
			continue
		}
		if namespace == shared.SpecialAnyOtherFlag {
			oldStateResponse := invokeSpecialAnyOtherFlag(ctx, k, evicted, deploymentStateByNamespace)
			return oldStateResponse
		}
		deploymentAndReplicasFingerprint := downscaleNamespace(ctx, k, namespace)
		deploymentStateByNamespace[namespace] = deploymentAndReplicasFingerprint
	}
	return deploymentStateByNamespace
}

func invokeSpecialAnyOtherFlag(ctx context.Context, k8sClient Kubernetes, evicted shared.NotUsableNamespacesDuringScheduling, oldState map[string][]string) map[string][]string {
	clusterNamespaces := k8sClient.GetNamespaces(ctx)
	for _, clusterNamespace := range clusterNamespaces {
		if isNamespaceIgnored(clusterNamespace, evicted) {
			continue
		}
		if isNamespaceAlreadyScheduled(clusterNamespace, evicted) {
			continue
		}
		if isCurrentNamespaceScheduledToDownscale(clusterNamespace, shared.DownscalerNamespace) {
			continue
		}
		deploymentAndReplicasFringerprint := downscaleNamespace(ctx, k8sClient, clusterNamespace)
		oldState[shared.SpecialAnyOtherFlag] = deploymentAndReplicasFringerprint
	}
	downscaleTheDownscaler(ctx, k8sClient, evicted)
	return oldState
}
