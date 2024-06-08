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
	GetNamespaces() []string
	GetConfigMap(ctx context.Context, name, namespace string) (*corev1.ConfigMap, error)
	GetWatcherByConfigMapName(ctx context.Context, name, namespace string) (watch.Interface, error)
	GetDeployments(ctx context.Context, namespace string) (*v1.DeploymentList, error)
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

func (kuberneterActor KubernetesHelperImpl) GetNamespaces() []string {
	return nil
}

func (kuberneterActor KubernetesHelperImpl) GetDeployments(ctx context.Context, namespace string) (*v1.DeploymentList, error) {
	deployments, err := kuberneterActor.K8sClient.AppsV1().
		Deployments(namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return deployments, nil
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
	timeout := int64(60)
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
