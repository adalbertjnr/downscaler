package shared

const (
	Version  = "v1"
	Resource = "downscalers"
	Group    = "scheduler.go"

	DownscalerNamespace = "downscaler"
	Unspecified         = "unspecified"
)

type TaskControl int

const (
	InspectError TaskControl = -1

	KillCurrentRoutine TaskControl = iota
	RestartRoutine

	DeploymentsWithDownscaledState
	DeploymentsWithUpscaledState

	AppStartupWithNoDataWrite

	UpscalingDeactivated
)

type Apps struct {
	Namespace                       string   `yaml:"namespace"`
	DeploymentsWithReplicasAndState []string `yaml:"deploymentsWithReplicasAndState"`
}

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

type NotUsableNamespacesDuringScheduling struct {
	IgnoredNamespaces   map[string]struct{}
	ScheduledNamespaces map[string]struct{}
}

type OldStateHelper struct {
	OldStateGeneric              map[string][]string
	DeploymentAndCurrentReplicas []string
}
