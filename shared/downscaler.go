package shared

const (
	Version  = "v1"
	Resource = "downscalers"
	Group    = "scheduler.go"

	AnyOther = "any-other"
)

type Metadata struct {
	Name      string
	Namespace string
}

type DownscalerPolicy struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	}
	Spec struct {
		ExecutionOpts struct {
			Time struct {
				TimeZone   string `yaml:"timeZone"`
				Recurrence string `yaml:"recurrence"`
				Downscaler struct {
					DownscalerSelectorTerms struct {
						MatchExpressions struct {
							Key      string   `yaml:"key"`
							Operator string   `yaml:"operator"`
							Values   []string `yaml:"values"`
						} `yaml:"matchExpressions"`
					} `yaml:"downscalerSelectorTerms"`
					WithNamespaceOpts struct {
						DownscaleNamespacesWithTimeRules struct {
							Rules []struct {
								Namespaces []string `yaml:"namespaces"`
								WithCron   string   `yaml:"withCron"`
							} `yaml:"rules"`
						} `yaml:"downscaleNamespacesWithTimeRules"`
					} `yaml:"withNamespaceOpts"`
				} `yaml:"downscaler"`
			} `yaml:"time"`
		} `yaml:"executionOpts"`
	} `yaml:"spec"`
}
