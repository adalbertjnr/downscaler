package helpers

import (
	"log/slog"
	"os"
	"strings"

	"github.com/adalbertjnr/downscaler/core"
	"github.com/adalbertjnr/downscaler/shared"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

type Helpers struct{}

func New() *Helpers {
	return &Helpers{}
}

func (h *Helpers) CurrentNamespace() string {
	namespace, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		panic(err)
	}
	slog.Info("found current namespace", "namespace", string(namespace))
	return string(namespace)
}

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
