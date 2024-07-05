package shared

import "log/slog"

type DownscalerExpression struct {
	MatchExpressions struct {
		Key      string   `yaml:"key"`
		Operator string   `yaml:"operator"`
		Values   []string `yaml:"values"`
	} `yaml:"matchExpressions"`
}

func (v *DownscalerExpression) WithExclude() bool {
	return v.MatchExpressions.Operator == "exclude"
}

func (v *DownscalerExpression) ShowIgnoredNamespaces(ignoredNamespaces []string) map[string]struct{} {
	if len(ignoredNamespaces) > 0 {
		excludeNamespaceMap := make(map[string]struct{})
		for _, namespace := range ignoredNamespaces {
			slog.Info("namespace validator",
				"namespace", namespace,
				"status", "ignored during crontime scheduling routine",
			)
			if _, exists := excludeNamespaceMap[namespace]; !exists {
				excludeNamespaceMap[namespace] = struct{}{}
			} else {
				continue
			}
		}
		return excludeNamespaceMap
	}
	slog.Info("namespace validator",
		"message", "no namespace was provided to be ignored during scheduling",
	)
	return nil
}

type DownscalerRules struct {
	Rules []struct {
		Namespaces []string `yaml:"namespaces"`
		WithCron   string   `yaml:"withCron"`
	} `yaml:"rules"`
}

func (v *DownscalerRules) Available() bool {
	return len(v.Rules) > 0
}
