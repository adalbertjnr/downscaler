package types

import (
	"os"

	"gopkg.in/yaml.v3"
)

func MustParseCmYaml(path string) *CmManifest {
	cm, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	cmManifest := &CmManifest{}
	if err := yaml.Unmarshal(cm, cmManifest); err != nil {
		return &CmManifest{}
	}

	return cmManifest
}

type CmManifest struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Data struct {
		PolicyYaml string `yaml:"policy.yaml"`
	} `yaml:"data"`
}

type DeschedulerPolicy struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Spec       struct {
		ExecutionOpts struct {
			Time struct {
				DefaultUptime string `yaml:"defaultUptime"`
				TimeZone      string `yaml:"timeZone"`
				Recurrence    string `yaml:"recurrence"`
			} `yaml:"time"`
			Downscaler struct {
				DownscalerSelectorTerms struct {
					MatchExpressions []struct {
						Key      string   `yaml:"key"`
						Operator string   `yaml:"operator"`
						Values   []string `yaml:"values"`
					} `yaml:"matchExpressions"`
				} `yaml:"downscalerSelectorTerms"`
			} `yaml:"downscaler"`
		} `yaml:"executionOpts"`
	} `yaml:"spec"`
}
