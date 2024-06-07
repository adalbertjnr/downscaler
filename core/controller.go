package core

import (
	"fmt"

	"github.com/adalbertjnr/downscaler/cron"
	"github.com/adalbertjnr/downscaler/input"
	"github.com/adalbertjnr/downscaler/k8sutil"
	"github.com/adalbertjnr/downscaler/types"
	"github.com/adalbertjnr/downscaler/watch"
	"gopkg.in/yaml.v2"

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

	data := &types.DownscalerPolicy{}
	err := unmarshalDataPolicy(cm, data)
	if err != nil {
		panic(err)
	}

	c.cron.AddYamlPolicy(data)
	watcher, err := c.client.GetWatcherByConfigMapName(cm.Metadata.Name, cm.Metadata.Namespace)
	if err != nil {
		panic(err)
	}

	go watch.ConfigMap(watcher, c.rtObjectch)
	go c.ReceiveNewConfigMapData()
	go cron.NewCron().
		MustAddLocation(data.Spec.ExecutionOpts.Time.TimeZone).
		StartCron()
}

func (c *Controller) updateNewCronLoop() {
	for {
		if cmDataPolicy, ok := <-c.cmObjectch; ok {
			c.cron.AddYamlPolicy(&cmDataPolicy)
			continue
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
				err := unmarshalDataPolicy(cm, data)
				if err != nil {
					fmt.Println(err)
				}
				c.cmObjectch <- *data
			}
		}
	}
}

func unmarshalDataPolicy(cm interface{}, data *types.DownscalerPolicy) error {
	switch v := cm.(type) {
	case *types.CmManifest:
		return yaml.Unmarshal([]byte(v.Data.PolicyYaml), data)
	case *corev1.ConfigMap:
		if yamlPolicy, found := v.Data[YamlCmPolicy]; found {
			return yaml.Unmarshal([]byte(yamlPolicy), data)
		}
	}
	return fmt.Errorf("error with the configMap data")
}
