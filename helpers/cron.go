package helpers

import (
	"strings"

	"github.com/adalbertjnr/downscaler/core"
	"github.com/adalbertjnr/downscaler/shared"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
)

func (h *Helpers) Timezone(cm *corev1.ConfigMap) string {
	tz := &shared.DownscalerTime{}
	if data, ok := cm.Data[core.YamlCmPolicy]; ok {
		if err := yaml.Unmarshal([]byte(data), &tz); err != nil {
			panic(err)
		}
	}
	return strings.TrimSpace(tz.Spec.ExecutionOpts.Time.TimeZone)
}

func (h *Helpers) AndParseCronConfig(cm *corev1.ConfigMap) *shared.DownscalerPolicy {
	policy := &shared.DownscalerPolicy{}
	if data, ok := cm.Data[core.YamlCmPolicy]; ok {
		if err := yaml.Unmarshal([]byte(data), &policy); err != nil {
			panic(err)
		}
	}
	return policy
}
