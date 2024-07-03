package shared

const (
	Version  = "v1"
	Resource = "downscalers"
	Group    = "scheduler.go"

	EmptyNamespace    = "empty"
	NotEmptyNamespace = "not_empty"

	DownscalerNamespace = "downscaler"
	Unspecified         = "unspecified"

	DefaultGroup     = "default"
	UnspecifiedGroup = "unspecified"
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
	Status string   `yaml:"status"`
	Group  string   `yaml:"group"`
	State  []string `yaml:"state"`
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

func (n NotUsableNamespacesDuringScheduling) Validate(namespaces []string) bool {
	for _, namespace := range namespaces {
		if namespace == Unspecified {
			return true
		}
	}
	return false
}

func (n *NotUsableNamespacesDuringScheduling) ReplaceSpecialFlagWithNamespaces(clusterNamespaces []string, namespaces []string) []string {
	toReplaceWith := make([]string, 0)

	for _, clusterNamespace := range clusterNamespaces {
		if _, found := n.IgnoredNamespaces[clusterNamespace]; found {
			continue
		}
		if _, found := n.ScheduledNamespaces[clusterNamespace]; found {
			continue
		}

		toReplaceWith = append(toReplaceWith, clusterNamespace)
	}

	return toReplaceWith
}

type NotUsableNamespacesDuringScheduling struct {
	IgnoredNamespaces   map[string]struct{}
	ScheduledNamespaces map[string]struct{}
}

type OldStateHelper struct {
	OldStateGeneric              map[string][]string
	DeploymentAndCurrentReplicas []string
}
