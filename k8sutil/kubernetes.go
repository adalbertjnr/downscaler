package k8sutil

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

type KubernetesHelper interface {
	GetNamespaces() []string
	GetConfigMap(name, namespace string) (*corev1.ConfigMap, error)
	GetWatcherByConfigMapName(name, namespace string) (watch.Interface, error)
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

func (kuberneterActor KubernetesHelperImpl) GetConfigMap(name, namespace string) (*corev1.ConfigMap, error) {
	return kuberneterActor.K8sClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

func (kuberneterActor KubernetesHelperImpl) GetWatcherByConfigMapName(name, namespace string) (watch.Interface, error) {
	watcher, err := kuberneterActor.K8sClient.CoreV1().ConfigMaps(namespace).Watch(context.Background(), metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name).String(),
	})
	if err != nil {
		return nil, err
	}
	return watcher, nil
}
