package helpers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/adalbertjnr/downscaler/shared"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func UnmarshalDataPolicy(cm interface{}, data interface{}, dataType ...string) error {
	switch v := cm.(type) {
	case *corev1.ConfigMap:
		switch strings.Join(dataType, "") {
		case shared.DataTypeDeployments:
			err := yaml.Unmarshal([]byte(v.Data[shared.DataTypeDeployments]), data.(map[string][]string))
			if err != nil {
				return err
			}
		case shared.DataTypeTimeHour:
			err := yaml.Unmarshal([]byte(v.Data[shared.DataTypeDeployments]), data.(map[string]string))
			if err != nil {
				return err
			}
		}
	case *unstructured.Unstructured:
		jsonData, err := json.Marshal(v.Object)
		if err != nil {
			return err
		}
		return yaml.Unmarshal([]byte(jsonData), data.(*shared.DownscalerPolicy))
	}
	return fmt.Errorf("error with the downscalercrd data")
}
