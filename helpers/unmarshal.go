package helpers

import (
	"encoding/json"
	"fmt"

	"github.com/adalbertjnr/downscaler/shared"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func UnmarshalDataPolicy(cm interface{}, data interface{}) error {
	switch v := cm.(type) {
	case *corev1.ConfigMap:
		jsonData, err := json.Marshal(v.Data)
		if err != nil {
			return err
		}
		return yaml.Unmarshal(jsonData, data.(map[string]interface{}))
	case *unstructured.Unstructured:
		jsonData, err := json.Marshal(v.Object)
		if err != nil {
			return err
		}
		return yaml.Unmarshal([]byte(jsonData), data.(*shared.DownscalerPolicy))
	}
	return fmt.Errorf("error with the downscalercrd data")
}
