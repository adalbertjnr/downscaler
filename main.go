package main

import (
	"descheduler/core"
	"descheduler/input"
	"descheduler/k8sutil"
	"descheduler/kubeclient"
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
