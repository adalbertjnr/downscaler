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
	ctx := context.Background()

	retrieve := helpers.New()
	currentNamespace := retrieve.CurrentNamespace()

	kubeApiSvc := k8sutil.NewKubernetesHelper(client)
	cm, err := kubeApiSvc.GetConfigMap(ctx, initialDefaultInput.InitialCmConfig, currentNamespace)
	if err != nil {
		panic(err)
	}

	cmMetadata := shared.Metadata{
		Name:      cm.Name,
		Namespace: cm.Namespace,
	}

	currentTz := retrieve.Timezone(cm)
	cronConfig := retrieve.AndParseCronConfig(cm)

	cronSvc := cron.NewCron().
		AddCronDetails(cronConfig).
		MustAddTimezoneLocation(currentTz)

	svc := core.NewController(ctx, kubeApiSvc, cronSvc, initialDefaultInput)
	svc.InitCmWatcher(ctx, cmMetadata)

	go svc.HandleSignals()

	svc.StartDownscaler()
}
