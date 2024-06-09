package shared

type DownscalerTime struct {
	Spec struct {
		ExecutionOpts struct {
			Time struct {
				TimeZone   string `yaml:"timeZone"`
				Recurrence string `yaml:"recurrence"`
			} `yaml:"time"`
		} `yaml:"executionOpts"`
	} `yaml:"spec"`
}
