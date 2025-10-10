package model

// PrometheusRule Prometheus规则文件中的单个规则
type PrometheusRule struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// PrometheusRuleGroup Prometheus规则组
type PrometheusRuleGroup struct {
	Name  string           `yaml:"name"`
	Rules []PrometheusRule `yaml:"rules"`
}

// PrometheusRuleFile Prometheus规则文件结构
type PrometheusRuleFile struct {
	Groups []PrometheusRuleGroup `yaml:"groups"`
}
