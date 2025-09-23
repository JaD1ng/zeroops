package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/qiniu/zeroops/internal/prometheus_adapter/client"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/model"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// AlertService 告警服务 - 仅负责与Prometheus交互，不存储规则
type AlertService struct {
	promClient    *client.PrometheusClient
	rulesFilePath string
}

// NewAlertService 创建告警服务
func NewAlertService(promClient *client.PrometheusClient) *AlertService {
	rulesFilePath := os.Getenv("PROMETHEUS_RULES_FILE")
	if rulesFilePath == "" {
		rulesFilePath = "/etc/prometheus/rules/alert_rules.yml"
	}

	return &AlertService{
		promClient:    promClient,
		rulesFilePath: rulesFilePath,
	}
}

// SyncRulesToPrometheus 同步规则到Prometheus
// 接收完整的规则列表，生成Prometheus规则文件并重载配置
func (s *AlertService) SyncRulesToPrometheus(rules []model.AlertRule, ruleMetas []model.AlertRuleMeta) error {
	// 构建Prometheus规则文件
	prometheusRules := s.buildPrometheusRules(rules, ruleMetas)

	// 写入规则文件
	if err := s.writeRulesFile(prometheusRules); err != nil {
		return fmt.Errorf("failed to write rules file: %w", err)
	}

	// 通知Prometheus重新加载配置
	if err := s.reloadPrometheus(); err != nil {
		log.Warn().Err(err).Msg("Failed to reload Prometheus, rules file has been updated")
		// 不返回错误，因为文件已经更新成功
	}

	log.Info().
		Int("rules_count", len(rules)).
		Int("metas_count", len(ruleMetas)).
		Msg("Rules synced to Prometheus successfully")

	return nil
}

// buildPrometheusRules 构建Prometheus规则
func (s *AlertService) buildPrometheusRules(rules []model.AlertRule, ruleMetas []model.AlertRuleMeta) *model.PrometheusRuleFile {
	promRules := []model.PrometheusRule{}

	// 创建规则名到规则的映射
	ruleMap := make(map[string]*model.AlertRule)
	for i := range rules {
		ruleMap[rules[i].Name] = &rules[i]
	}

	// 为每个元信息生成Prometheus规则
	for _, meta := range ruleMetas {
		// 查找对应的规则模板
		var rule *model.AlertRule

		// 尝试从alert_name中提取规则名
		// 假设alert_name格式为: {rule_name}_{service}_{version} 或类似格式
		for ruleName, r := range ruleMap {
			if strings.HasPrefix(meta.AlertName, ruleName) {
				rule = r
				break
			}
		}

		if rule == nil {
			log.Warn().
				Str("alert_name", meta.AlertName).
				Msg("No matching rule template found for alert meta, skipping")
			continue
		}

		// 解析标签
		var labels map[string]string
		if meta.Labels != "" {
			if err := json.Unmarshal([]byte(meta.Labels), &labels); err != nil {
				log.Warn().
					Err(err).
					Str("alert_name", meta.AlertName).
					Msg("Failed to parse labels, using empty labels")
				labels = make(map[string]string)
			}
		} else {
			labels = make(map[string]string)
		}

		// 添加severity标签
		labels["severity"] = rule.Severity
		labels["rule_name"] = rule.Name

		// 构建表达式
		expr := s.buildExpression(rule, &meta)

		// 构建注释
		annotations := map[string]string{
			"description": rule.Description,
			"summary":     fmt.Sprintf("%s %s %f", rule.Expr, rule.Op, meta.Threshold),
		}

		// 计算for字段
		forDuration := ""
		if meta.WatchTime > 0 {
			forDuration = fmt.Sprintf("%ds", meta.WatchTime)
		}

		promRule := model.PrometheusRule{
			Alert:       meta.AlertName,
			Expr:        expr,
			For:         forDuration,
			Labels:      labels,
			Annotations: annotations,
		}

		promRules = append(promRules, promRule)
	}

	// 如果没有元信息，为每个规则创建默认规则
	if len(ruleMetas) == 0 {
		for _, rule := range rules {
			labels := map[string]string{
				"severity": rule.Severity,
			}

			annotations := map[string]string{
				"description": rule.Description,
				"summary":     fmt.Sprintf("%s triggered", rule.Name),
			}

			promRule := model.PrometheusRule{
				Alert:       rule.Name,
				Expr:        rule.Expr,
				Labels:      labels,
				Annotations: annotations,
			}

			promRules = append(promRules, promRule)
		}
	}

	return &model.PrometheusRuleFile{
		Groups: []model.PrometheusRuleGroup{
			{
				Name:  "zeroops_alerts",
				Rules: promRules,
			},
		},
	}
}

// buildExpression 构建PromQL表达式
func (s *AlertService) buildExpression(rule *model.AlertRule, meta *model.AlertRuleMeta) string {
	expr := rule.Expr

	// 解析标签并添加到表达式中
	var labels map[string]string
	if meta.Labels != "" {
		json.Unmarshal([]byte(meta.Labels), &labels)
	}

	if len(labels) > 0 {
		labelMatchers := []string{}
		for k, v := range labels {
			// 跳过内部使用的标签
			if k == "rule_name" {
				continue
			}
			labelMatchers = append(labelMatchers, fmt.Sprintf(`%s="%s"`, k, v))
		}

		if len(labelMatchers) > 0 {
			// 如果表达式包含{，说明已经有标签选择器
			if strings.Contains(expr, "{") {
				expr = strings.Replace(expr, "}", ","+strings.Join(labelMatchers, ",")+"}", 1)
			} else {
				// 在指标名后添加标签选择器
				// 查找第一个非字母数字下划线的字符
				metricEnd := 0
				for i, ch := range expr {
					if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
						(ch >= '0' && ch <= '9') || ch == '_') {
						metricEnd = i
						break
					}
				}
				if metricEnd == 0 {
					metricEnd = len(expr)
				}
				expr = expr[:metricEnd] + "{" + strings.Join(labelMatchers, ",") + "}" + expr[metricEnd:]
			}
		}
	}

	// 添加时间范围
	if meta.MatchTime != "" {
		// 查找最后一个指标，添加时间范围
		if !strings.Contains(expr, "[") {
			// 简单处理：在第一个空格前添加时间范围
			parts := strings.SplitN(expr, " ", 2)
			if len(parts) == 2 {
				expr = parts[0] + "[" + meta.MatchTime + "] " + parts[1]
			} else {
				expr = expr + "[" + meta.MatchTime + "]"
			}
		}
	}

	// 添加比较操作符和阈值
	if meta.Threshold != 0 {
		expr = fmt.Sprintf("%s %s %f", expr, rule.Op, meta.Threshold)
	}

	return expr
}

// writeRulesFile 写入规则文件
func (s *AlertService) writeRulesFile(rules *model.PrometheusRuleFile) error {
	// 确保目录存在
	dir := filepath.Dir(s.rulesFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create rules directory: %w", err)
	}

	// 序列化为YAML
	data, err := yaml.Marshal(rules)
	if err != nil {
		return fmt.Errorf("failed to marshal rules: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(s.rulesFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write rules file: %w", err)
	}

	log.Info().
		Str("file", s.rulesFilePath).
		Int("groups", len(rules.Groups)).
		Msg("Prometheus rules file updated")

	return nil
}

// reloadPrometheus 重新加载Prometheus配置
func (s *AlertService) reloadPrometheus() error {
	prometheusURL := os.Getenv("PROMETHEUS_ADDRESS")
	if prometheusURL == "" {
		prometheusURL = "http://10.210.10.33:9090"
	}

	reloadURL := fmt.Sprintf("%s/-/reload", strings.TrimSuffix(prometheusURL, "/"))

	resp, err := http.Post(reloadURL, "text/plain", nil)
	if err != nil {
		return fmt.Errorf("failed to reload Prometheus: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Prometheus reload failed with status: %d", resp.StatusCode)
	}

	log.Info().Msg("Prometheus configuration reloaded")
	return nil
}
