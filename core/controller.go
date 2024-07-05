package core

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/adalbertjnr/downscaler/helpers"
	"github.com/adalbertjnr/downscaler/input"
	"github.com/adalbertjnr/downscaler/kas"
	"github.com/adalbertjnr/downscaler/scheduler"
	"github.com/adalbertjnr/downscaler/shared"
	"github.com/adalbertjnr/downscaler/watcher"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type Controller struct {
	client            kas.Kubernetes
	scheduler         scheduler.Scheduler
	rtObjectch        chan runtime.Object
	cmObjectch        chan shared.DownscalerPolicy
	ctx               context.Context
	cancelFn          context.CancelFunc
	initialCronConfig *shared.DownscalerPolicy
	watch             *watcher.Watcher
	input             *input.FromArgs
}

func NewController(ctx context.Context,
	client kas.Kubernetes,
	scheduler *scheduler.Scheduler,
	initialCronConfig *shared.DownscalerPolicy,
	watch *watcher.Watcher,
	input *input.FromArgs,
) *Controller {
	context, cancel := context.WithCancel(ctx)
	return &Controller{
		client:            client,
		scheduler:         *scheduler,
		initialCronConfig: initialCronConfig,
		rtObjectch:        make(chan runtime.Object, 1),
		cmObjectch:        make(chan shared.DownscalerPolicy, 1),
		ctx:               context,
		cancelFn:          cancel,
		watch:             watch,
		input:             input,
	}
}

func (c *Controller) updateNewCronLoop() {
	for {
		select {
		case cmDataPolicy := <-c.cmObjectch:
			c.scheduler.AddSchedulerDetails(&cmDataPolicy)
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Controller) StartDownscaler() {
	go c.ReceiveNewConfigMapData()
	go c.updateNewCronLoop()
	go c.scheduler.StartScheduler()

	slog.Info("downscaler initialization", "status", "initialized")

	c.cmObjectch <- *c.initialCronConfig
	<-c.ctx.Done()
	slog.Warn("the downscaler is shutting down gracefully...")
}

func (c *Controller) ReceiveNewConfigMapData() {
	for {
		select {
		case object := <-c.watch.RtObjectch:
			cm, converted := object.(*unstructured.Unstructured)
			if converted {
				data := &shared.DownscalerPolicy{}
				err := helpers.UnmarshalDataPolicy(cm, data)
				if err != nil {
					slog.Error("error unmarshaling the yaml data policy", "error", err.Error())
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
	slog.Warn("the downscaler received a signal to be terminated")
	c.cancelFn()
}

func (c *Controller) ValidateConfigMapInitialization() {
	if c.input.RunUpscaling {
		cm := c.client.ListConfigMap(c.ctx, c.input.ConfigMapName, c.input.ConfigMapNamespace)
		if cm != nil {
			return
		}
		err := c.client.CreateConfigMap(c.ctx, c.input.ConfigMapName, c.input.ConfigMapNamespace)
		if err != nil {
			return
		}
	}
}
