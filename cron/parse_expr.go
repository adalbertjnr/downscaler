package cron

import (
	"github.com/adalbertjnr/downscaler/shared"
)

type ValidateDownscalerExpr struct {
	MatchExpressions struct {
		Key      string   `yaml:"key"`
		Operator string   `yaml:"operator"`
		Values   []string `yaml:"values"`
	} `yaml:"matchExpressions"`
}

func ParseExpr(downscalerData *shared.DownscalerPolicy) ValidateDownscalerExpr {
	downscaler := downscalerData.Spec.ExecutionOpts.Time.Downscaler
	data := ValidateDownscalerExpr{
		MatchExpressions: downscaler.DownscalerSelectorTerms.MatchExpressions,
	}

	return data
}

func (v *ValidateDownscalerExpr) withExclude() bool {
	return v.MatchExpressions.Operator == "exclude"
}

type ValidateDownscalerCriteria struct {
	Criteria []struct {
		Namespaces []string `yaml:"namespaces"`
		WithCron   string   `yaml:"withCron"`
	} `yaml:"criteria"`
}

func ParseCriteria(downscalerData *shared.DownscalerPolicy) ValidateDownscalerCriteria {
	downscaler := downscalerData.Spec.ExecutionOpts.Time.Downscaler
	data := ValidateDownscalerCriteria{
		Criteria: downscaler.WithAdvancedNamespaceOpts.MatchCriteria.Criteria,
	}

	return data
}

func (v *ValidateDownscalerCriteria) available() bool {
	return len(v.Criteria) > 0
}
