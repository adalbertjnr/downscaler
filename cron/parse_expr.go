package cron

type DownscalerExpression struct {
	MatchExpressions struct {
		Key      string   `yaml:"key"`
		Operator string   `yaml:"operator"`
		Values   []string `yaml:"values"`
	} `yaml:"matchExpressions"`
}

func (v *DownscalerExpression) withExclude() bool {
	return v.MatchExpressions.Operator == "exclude"
}

type DownscalerCriteria struct {
	Criteria []struct {
		Namespaces []string `yaml:"namespaces"`
		WithCron   string   `yaml:"withCron"`
	} `yaml:"criteria"`
}

func (v *DownscalerCriteria) available() bool {
	return len(v.Criteria) > 0
}
