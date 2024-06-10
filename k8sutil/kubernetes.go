package k8sutil

import (
	"context"
	"fmt"
	"log/slog"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

type KubernetesHelper interface {
	GetNamespaces(ctx context.Context) []string
	GetConfigMap(ctx context.Context, name, namespace string) (*corev1.ConfigMap, error)
	GetWatcherByConfigMapName(ctx context.Context, name, namespace string) (watch.Interface, error)
	GetDeployments(ctx context.Context, namespace string) *v1.DeploymentList
	Downscale(ctx context.Context, namespace string, deployment *v1.Deployment)
}

type KubernetesHelperImpl struct {
	K8sClient *kubernetes.Clientset
}

func NewKubernetesHelper(client *kubernetes.Clientset) *KubernetesHelperImpl {
	return &KubernetesHelperImpl{
		K8sClient: client,
	}
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

func (kuberneterActor KubernetesHelperImpl) Downscale(ctx context.Context, namespace string, deployment *v1.Deployment) {
	desiredReplicas := int32(0)
	slog.Info("downscaling deployment",
		"name", deployment.Name,
		"namespace", deployment.Namespace,
		"current replicas", deployment.Spec.Replicas,
		"desired replicas state", desiredReplicas,
	)

	deployment.Spec.Replicas = &desiredReplicas
	scaleResp, err := kuberneterActor.K8sClient.AppsV1().
		Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		slog.Error("downscaling error", "deployment", deployment.Name, "error", err.Error())
		return
	}
	slog.Info("downscale was done successfully",
		"name", deployment.Name,
		"namespace", deployment.Name,
		"current replicas", desiredReplicas,
	)
	fmt.Println("printing the scaleresp.name", scaleResp.Name)
}

func (kuberneterActor KubernetesHelperImpl) GetConfigMap(ctx context.Context, name, namespace string) (*corev1.ConfigMap, error) {
	return kuberneterActor.K8sClient.CoreV1().
		ConfigMaps(namespace).
		Get(ctx, name, metav1.GetOptions{})
}

func (kuberneterActor KubernetesHelperImpl) GetWatcherByConfigMapName(ctx context.Context, name, namespace string) (watch.Interface, error) {
	timeout := int64(3600)
	watcher, err := kuberneterActor.K8sClient.CoreV1().
		ConfigMaps(namespace).
		Watch(ctx, metav1.ListOptions{
			FieldSelector:  fields.OneTermEqualSelector("metadata.name", name).String(),
			TimeoutSeconds: &timeout,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to create the watcher. configMap name %s with namespace %s. err: %v", name, namespace, err)
	}
	slog.Info("watcher created successfully from configmap", "name", name, "namespace", namespace)
	return watcher, nil
}

func TriggerDownscaler(ctx context.Context, k8sClient KubernetesHelper, namespaces []string) {
	for _, namespace := range namespaces {
		deployments := k8sClient.GetDeployments(ctx, namespace)

		for _, deployment := range deployments.Items {
			k8sClient.Downscale(ctx, namespace, &deployment)
		}
	}
}
