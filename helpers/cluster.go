package helpers

import (
	"log/slog"
	"os"
	"strings"

	"github.com/adalbertjnr/downscaler/core"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

func GetCurrentNamespace() string {
	namespace, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		panic(err)
	}
	slog.Info("found current namespace", "namespace", string(namespace))
	return string(namespace)
}

type Tz struct {
	Spec struct {
		ExecutionOpts struct {
			Time struct {
				TimeZone string `yaml:"timeZone"`
			} `yaml:"time"`
		} `yaml:"executionOpts"`
	} `yaml:"spec"`
}

func RetrieveTzFromCm(cm *corev1.ConfigMap) string {
	tz := &Tz{}
	if data, ok := cm.Data[core.YamlCmPolicy]; ok {
		if err := yaml.Unmarshal([]byte(data), &tz); err != nil {
			panic(err)
		}
	}
	return strings.TrimSpace(tz.Spec.ExecutionOpts.Time.TimeZone)
}
