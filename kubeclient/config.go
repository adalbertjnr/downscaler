package kubeclient

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func NewClientOrDie() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientSet, nil
}
