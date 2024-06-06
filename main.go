package main

import (
	"github.com/adalbertjnr/downscaler/core"
	"github.com/adalbertjnr/downscaler/input"
	"github.com/adalbertjnr/downscaler/k8sutil"
	"github.com/adalbertjnr/downscaler/kubeclient"
)

func main() {
	initialDefaultInput := input.Flags()
	client, err := kubeclient.NewClientOrDie()
	if err != nil {
		panic(err)
	}

	defaultOperations := k8sutil.NewKubernetesHelper(client)

	svc := core.NewController(defaultOperations, initialDefaultInput)
	svc.InitCmWatcher()
}
