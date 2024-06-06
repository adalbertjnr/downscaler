package core

import (
	"log"

	"github.com/adalbertjnr/downscaler/cron"
	"github.com/adalbertjnr/downscaler/input"
	"github.com/adalbertjnr/downscaler/k8sutil"
	"github.com/adalbertjnr/downscaler/types"
	"github.com/adalbertjnr/downscaler/watch"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/runtime"
)

type Controller struct {
	client     k8sutil.KubernetesHelper
	cron       cron.Cron
	input      input.FromFlags
	rtObjectch chan runtime.Object
	cmObjectch chan types.DownscalerPolicy
}

func NewController(client k8sutil.KubernetesHelper, cron *cron.Cron, input *input.FromFlags) *Controller {
	return &Controller{
		client:     client,
		cron:       *cron,
		input:      *input,
		rtObjectch: make(chan runtime.Object),
		cmObjectch: make(chan types.DownscalerPolicy),
	}
}

const YamlCmPolicy = "policy.yaml"

func (c *Controller) InitCmWatcher() {
	cm := types.MustParseCmYaml(c.input.InitialCmConfig)
	watcher, err := c.client.GetWatcherByConfigMapName(cm.Metadata.Name, cm.Metadata.Namespace)
	if err != nil {
		panic(err)
	}

	go watch.ConfigMap(watcher, c.rtObjectch)
	go c.ReceiveNewConfigMapData()
}

func (c *Controller) updateNewCronLoop() {
	for {
		if cmDataPolicy, ok := <-c.cmObjectch; ok {
			c.cron.AddCron(cmDataPolicy.Spec.ExecutionOpts.Time.DefaultUptime)
		}
	}
}

func (c *Controller) StartDownscaler() {
	c.updateNewCronLoop()
}

func (c *Controller) ReceiveNewConfigMapData() {
	for {
		if object, received := <-c.rtObjectch; received {
			cm, converted := object.(*corev1.ConfigMap)
			if converted {
				data := &types.DownscalerPolicy{}

				if yamlPolicy, found := cm.Data[YamlCmPolicy]; found {
					err := yaml.Unmarshal([]byte(yamlPolicy), data)
					if err != nil {
						log.Println(err)
					}
					c.cmObjectch <- *data
				}
			}
		}
	}
}
