package common

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/adalbertjnr/downscaler/shared"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func UnmarshalDataPolicy(cm interface{}, data interface{}) error {
	switch v := cm.(type) {
	case *corev1.ConfigMap:
		for key, value := range v.Data {
			var app shared.Apps
			err := yaml.Unmarshal([]byte(value), &app)
			if err != nil {
				return err
			}
			data.(map[string]shared.Apps)[key] = app
		}
		return nil
	case *unstructured.Unstructured:
		jsonData, err := json.Marshal(v.Object)
		if err != nil {
			return err
		}
		return yaml.Unmarshal([]byte(jsonData), data.(*shared.DownscalerPolicy))
	}
	return fmt.Errorf("error with the downscalercrd data")
}

func RetrieveTzFromData(data *shared.DownscalerPolicy) string {
	return strings.TrimSpace(data.Spec.ExecutionOpts.Time.TimeZone)
}
