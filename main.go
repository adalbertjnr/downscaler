package main

import (
	"context"

	"github.com/adalbertjnr/downscaler/core"
	"github.com/adalbertjnr/downscaler/cron"
	"github.com/adalbertjnr/downscaler/helpers"
	"github.com/adalbertjnr/downscaler/input"
	"github.com/adalbertjnr/downscaler/k8sutil"
	"github.com/adalbertjnr/downscaler/kubeclient"
	"github.com/adalbertjnr/downscaler/shared"
)

func main() {
	initialDefaultInput := input.Flags()
	client, err := kubeclient.NewClientOrDie()
	if err != nil {
		panic(err)
	}
	currentNamespace := helpers.GetCurrentNamespace()

	kubeApiSvc := k8sutil.NewKubernetesHelper(client)
	cm, err := kubeApiSvc.GetConfigMap(initialDefaultInput.InitialCmConfig, currentNamespace)
	if err != nil {
		panic(err)
	}

	cmMetadata := shared.Metadata{
		Name:      cm.Name,
		Namespace: cm.Namespace,
	}

	currentTz := helpers.RetrieveTzFromCm(cm)
	cronSvc := cron.NewCron().
		MustAddTimezoneLocation(currentTz)

	ctx := context.Background()
	svc := core.NewController(ctx, kubeApiSvc, cronSvc, initialDefaultInput)
	svc.InitCmWatcher(cmMetadata)

	go svc.HandleSignals()

	svc.StartDownscaler()
}
