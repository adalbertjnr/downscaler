package main

import (
	"context"

	"github.com/adalbertjnr/downscaler/core"
	"github.com/adalbertjnr/downscaler/cron"
	"github.com/adalbertjnr/downscaler/helpers"
	"github.com/adalbertjnr/downscaler/k8sutil"
	"github.com/adalbertjnr/downscaler/kubeclient"
	"github.com/adalbertjnr/downscaler/shared"
	"github.com/adalbertjnr/downscaler/watcher"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func main() {
	client, err := kubeclient.NewClientOrDie()
	if err != nil {
		panic(err)
	}
	dynamicClient, err := kubeclient.NewDynamicClientOrDie()
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	retrieve := helpers.New()

	scm := schema.GroupVersionResource{
		Version:  shared.Version,
		Resource: shared.Resource,
		Group:    shared.Group,
	}

	kubeApiSvc := k8sutil.NewKubernetesHelper(client, dynamicClient)

	policyData, err := kubeApiSvc.GetDownscalerData(ctx, scm)
	if err != nil {
		panic(err)
	}

	cmMetadata := shared.Metadata{
		Name: policyData.Metadata.Name,
	}

	watch := watcher.New()
	go watch.DownscalerKind(ctx, cmMetadata, kubeApiSvc)

	currentTz := retrieve.Timezone(policyData)

	cronSvc := cron.NewCron().
		MustAddTimezoneLocation(currentTz).
		AddKubeApiSvc(kubeApiSvc)

	svc := core.NewController(ctx,
		kubeApiSvc,
		cronSvc,
		policyData,
		watch,
	)

	go svc.HandleSignals()

	svc.StartDownscaler()
}
