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
	ScaleDeployments(ctx context.Context, namespace string, deployment *v1.Deployment, patch []byte, updateScale int32)
	GetWatcherByDownscalerCRD(ctx context.Context, name, namespace string) (watch.Interface, error)
	StartDownscaling(ctx context.Context, namespaces []string, is shared.NotUsableNamespacesDuringScheduling) map[string]shared.Apps
	StartUpscaling(ctx context.Context, namespaces []string, cmName, cmNamespace string) map[string]shared.Apps
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
		slog.Error("configmap", "name", name, "namespace", namespace, "verb", "create", "err", err)
		return err
	}

	slog.Info("configmap", "name", name, "namespace", namespace, "verb", "create", "err", nil)
	return nil
}

func (k KubernetesImpl) ListConfigMap(ctx context.Context, name, namespace string) *corev1.ConfigMap {
	cm, err := k.K8sClient.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name).String(),
	},
	)
	if err != nil {
		slog.Error("configmap", "name", name, "namespace", namespace, "verb", "list", "err", err)
	}
	if len(cm.Items) > 0 {
		return &cm.Items[0]
	}

	return nil
}

func (k KubernetesImpl) PatchConfigMap(ctx context.Context, name, namespace string, patch []byte) {
	_, err := k.K8sClient.CoreV1().ConfigMaps(namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		slog.Error("configmap", "name", name, "namespace", namespace, "verb", "patch", "err", err)
	}
}

func (k KubernetesImpl) GetDownscalerData(ctx context.Context, gv schema.GroupVersionResource) (*shared.DownscalerPolicy, error) {
	list, err := k.DynamicClient.Resource(gv).Namespace("").List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", "downscaler").String(),
	})
	if err != nil {
		slog.Error("crd", "kind", "downscaler", "verb", "list", "err", err)
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
		slog.Error("namespace", "verb", "list", "error", err)
		return nil
	}

	namespacesNames := make([]string, len(namespaces.Items))
	for i, namespace := range namespaces.Items {
		namespacesNames[i] = namespace.Name
	}

	return namespacesNames
}

func (k KubernetesImpl) GetDeployments(ctx context.Context, namespace string) *v1.DeploymentList {
	deployments, err := k.K8sClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Error("deployments", "verb", "list", "namespace", namespace, "err", err)
		return nil
	}

	return deployments
}

func (k KubernetesImpl) ScaleDeployments(ctx context.Context, namespace string, deployment *v1.Deployment, patch []byte, desiredReplicas int32) {
	currentReplicas := *deployment.Spec.Replicas

	_, err := k.K8sClient.AppsV1().Deployments(namespace).Patch(ctx, deployment.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		slog.Error("deployments", "name", deployment.Name, "namespace", namespace, "current replicas", currentReplicas, "desired replicas", desiredReplicas, "verb", "update", "err", err)
		return
	}

	slog.Info("deployments", "name", deployment.Name, "namespace", namespace, "current replicas", currentReplicas, "desired replicas", desiredReplicas, "verb", "update", "err", err)
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
		"verb", "watch",
		"status", "created",
	)
	return watcher, nil
}

func (k KubernetesImpl) StartUpscaling(ctx context.Context, namespaces []string, cmName, cmNamespace string) map[string]shared.Apps {
	cm := k.ListConfigMap(ctx, cmName, cmNamespace)

	apps := make(map[string]shared.Apps)
	if err := helpers.UnmarshalDataPolicy(cm, apps); err != nil {
		slog.Error("unmarshal cm apps", "err", err)
	}
	deploymentMapList := filterDeploymentsByNamespace(ctx, namespaces, k)

	for _, namespace := range namespaces {
		if cmValue, found := apps[namespace+".yaml"]; found {
			for _, cmStoredState := range cmValue.State {
				cmDeploymentName, cmDeploymentReplicas := getMetadataReplicas(cmStoredState)
				patch, err := generateScalePatch(cmDeploymentReplicas)
				if err != nil {
					slog.Error("generating patch error", "err", err)
					continue
				}

				deployment := deploymentMapList[cmDeploymentName]
				k.ScaleDeployments(ctx, namespace, deployment, patch, cmDeploymentReplicas)
			}
		}
	}

	return apps
}

func (k KubernetesImpl) StartDownscaling(ctx context.Context, namespaces []string, evicted shared.NotUsableNamespacesDuringScheduling,
) map[string]shared.Apps {
	deploymentStateByNamespace := make(map[string]shared.Apps)
	for _, namespace := range namespaces {
		if isNamespaceIgnored(namespace, evicted) {
			continue
		}
		if namespace == shared.Unspecified {
			cmCurrentState := invokeUnspecifiedNamespaces(ctx, k, evicted, deploymentStateByNamespace)
			return cmCurrentState
		}
		deploymentAndReplicasFingerprint, _ := downscaleNamespace(ctx, k, namespace, shared.DefaultGroup)
		deploymentStateByNamespace[namespace] = deploymentAndReplicasFingerprint
	}
	return deploymentStateByNamespace
}

func invokeUnspecifiedNamespaces(ctx context.Context, k8sClient Kubernetes, evictedNamespaces shared.NotUsableNamespacesDuringScheduling, cmCurrentState map[string]shared.Apps) map[string]shared.Apps {
	clusterNamespaces := k8sClient.GetNamespaces(ctx)
	for _, clusterNamespace := range clusterNamespaces {
		if shouldSkipNamespace(clusterNamespace, evictedNamespaces) {
			continue
		}
		deploymentAndReplicasFringerprint, err := downscaleNamespace(ctx, k8sClient, clusterNamespace, shared.UnspecifiedGroup)
		if err != nil {
			continue
		}
		cmCurrentState[clusterNamespace] = deploymentAndReplicasFringerprint
	}
	downscaleTheDownscaler(ctx, k8sClient, evictedNamespaces)
	return cmCurrentState
}
