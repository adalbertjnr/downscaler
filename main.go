package main

import (
	"context"

	"github.com/adalbertjnr/downscaler/core"
	"github.com/adalbertjnr/downscaler/input"
	"github.com/adalbertjnr/downscaler/internal/common"
	"github.com/adalbertjnr/downscaler/kas"
	"github.com/adalbertjnr/downscaler/kubeclient"
	"github.com/adalbertjnr/downscaler/scheduler"
	"github.com/adalbertjnr/downscaler/shared"
	"github.com/adalbertjnr/downscaler/watcher"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func main() {
	args := input.FromEntrypoint()

	client, err := kubeclient.NewClientOrDie()
	if err != nil {
		panic(err)
	}
	dynamicClient, err := kubeclient.NewDynamicClientOrDie()
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	scm := schema.GroupVersionResource{
		Version:  shared.Version,
		Resource: shared.Resource,
		Group:    shared.Group,
	}

	kubeApiSvc := kas.NewKubernetes(client, dynamicClient)

	policyData, err := kubeApiSvc.GetDownscalerData(ctx, scm)
	if err != nil {
		panic(err)
	}

	cmMetadata := shared.Metadata{
		Name: policyData.Metadata.Name,
	}

	watch := watcher.New()
	go watch.DownscalerKind(ctx, cmMetadata, kubeApiSvc)

	currentTz := common.RetrieveTzFromData(policyData)

	schedulerSvc := scheduler.NewScheduler().
		MustAddTimezoneLocation(currentTz).
		AddKubeApiSvc(kubeApiSvc).
		AddInput(args)

	svc := core.NewController(ctx, kubeApiSvc, schedulerSvc, policyData, watch, args)

	go svc.HandleSignals()

	svc.ValidateConfigMapInitialization()
	svc.StartDownscaler()
}
