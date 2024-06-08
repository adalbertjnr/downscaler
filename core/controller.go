package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/adalbertjnr/downscaler/cron"
	"github.com/adalbertjnr/downscaler/input"
	"github.com/adalbertjnr/downscaler/k8sutil"
	"github.com/adalbertjnr/downscaler/shared"
	"github.com/adalbertjnr/downscaler/watch"
	"gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/runtime"
)

type Controller struct {
	client     k8sutil.KubernetesHelper
	cron       cron.Cron
	rtObjectch chan runtime.Object
	cmObjectch chan shared.DownscalerPolicy
	ctx        context.Context
	cancelFn   context.CancelFunc
}

func NewController(ctx context.Context, client k8sutil.KubernetesHelper, cron *cron.Cron, input *input.FromFlags) *Controller {
	context, cancel := context.WithCancel(ctx)
	return &Controller{
		client:     client,
		cron:       *cron,
		rtObjectch: make(chan runtime.Object),
		cmObjectch: make(chan shared.DownscalerPolicy),
		ctx:        context,
		cancelFn:   cancel,
	}
}

const YamlCmPolicy = "policy.yaml"

func (c *Controller) InitCmWatcher(ctx context.Context, cmMetadata shared.Metadata) {
	watcher, err := c.client.GetWatcherByConfigMapName(
		ctx,
		cmMetadata.Name,
		cmMetadata.Namespace,
	)
	if err != nil {
		panic(err)
	}
	go watch.ConfigMap(ctx, c.client, watcher, c.rtObjectch)
}

func (c *Controller) updateNewCronLoop() {
	for {
		select {
		case cmDataPolicy := <-c.cmObjectch:
			c.cron.AddYamlPolicy(&cmDataPolicy)
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Controller) StartDownscaler() {
	go c.ReceiveNewConfigMapData()
	go c.updateNewCronLoop()
	go c.cron.StartCron()
	<-c.ctx.Done()
	slog.Warn("the downscaler is shuting down gracefully")
}

func (c *Controller) ReceiveNewConfigMapData() {
	for {
		select {
		case object := <-c.rtObjectch:
			cm, converted := object.(*corev1.ConfigMap)
			if converted {
				data := &shared.DownscalerPolicy{}
				err := unmarshalDataPolicy(cm, data)
				if err != nil {
					fmt.Println(err)
				}
				c.cmObjectch <- *data
			}
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Controller) HandleSignals() {
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)

	<-sigch
	slog.Warn("the downscaler received a sigterm signal")
	c.cancelFn()
}

func unmarshalDataPolicy(cm interface{}, data *shared.DownscalerPolicy) error {
	switch v := cm.(type) {
	case *shared.CmManifest:
		return yaml.Unmarshal([]byte(v.Data.PolicyYaml), data)
	case *corev1.ConfigMap:
		if yamlPolicy, found := v.Data[YamlCmPolicy]; found {
			return yaml.Unmarshal([]byte(yamlPolicy), data)
		}
	}
	return fmt.Errorf("error with the configMap data")
}
