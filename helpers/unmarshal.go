package helpers

import (
	"encoding/json"
	"fmt"

	"github.com/adalbertjnr/downscaler/shared"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func UnmarshalDataPolicy(cm interface{}, data *shared.DownscalerPolicy) error {
	switch v := cm.(type) {
	case unstructured.Unstructured:
		jsonData, err := json.Marshal(v.Object)
		if err != nil {
			return err
		}
		return yaml.Unmarshal([]byte(jsonData), data)
	case *unstructured.Unstructured:
		jsonData, err := json.Marshal(v.Object)
		if err != nil {
			return err
		}
		return yaml.Unmarshal([]byte(jsonData), data)
	}
	return fmt.Errorf("error with the downscalercrd data")
}
