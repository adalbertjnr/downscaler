package core

import (
	"descheduler/input"
	"descheduler/k8sutil"
	"descheduler/types"
	"descheduler/watch"
	"fmt"
	"log"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/runtime"
)

type Controller struct {
	Client     k8sutil.KubernetesHelper
	input      input.FromFlags
	rtObjectch chan runtime.Object
	cmObjectch chan types.DeschedulerPolicy
}

func NewController(client k8sutil.KubernetesHelper, input *input.FromFlags) *Controller {
	return &Controller{
		Client:     client,
		input:      *input,
		rtObjectch: make(chan runtime.Object),
		cmObjectch: make(chan types.DeschedulerPolicy),
	}
}

func (c *Controller) InitCmWatcher() {
	cm := types.MustParseCmYaml(c.input.InitialCmConfig)
	watcher, err := c.Client.GetWatcherByConfigMapName(cm.Metadata.Name, cm.Metadata.Namespace)
	if err != nil {
		panic(err)
	}

	go watch.ConfigMap(watcher, c.rtObjectch)
	go c.ReceiveNewConfigMapData()
}

func (c *Controller) Receiving() {
	for data := range c.cmObjectch {
		fmt.Println(data)
	}
}

func (c *Controller) ReceiveNewConfigMapData() {
	if object, received := <-c.rtObjectch; received {
		cm, converted := object.(*corev1.ConfigMap)
		if converted {
			data := types.DeschedulerPolicy{}

			if yamlPolicy, found := cm.Data["policy.yaml"]; found {
				err := yaml.Unmarshal([]byte(yamlPolicy), data)
				if err != nil {
					log.Println(err)
				}
				c.cmObjectch <- data
			}
		}
	}
}
