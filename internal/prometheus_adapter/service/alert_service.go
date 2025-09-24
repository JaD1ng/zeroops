package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
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
	// 内存中缓存当前规则，用于增量更新
	currentRules     []model.AlertRule
	currentRuleMetas []model.AlertRuleMeta
}

// NewAlertService 创建告警服务
func NewAlertService(promClient *client.PrometheusClient) *AlertService {
	rulesFilePath := os.Getenv("PROMETHEUS_RULES_FILE")
	if rulesFilePath == "" {
		// 在本地生成规则文件，用于调试和后续同步到远程容器
		rulesFilePath = "./prometheus_rules/alert_rules.yml"
	}

	return &AlertService{
		promClient:       promClient,
		rulesFilePath:    rulesFilePath,
		currentRules:     []model.AlertRule{},
		currentRuleMetas: []model.AlertRuleMeta{},
	}
}

// SyncRulesToPrometheus 同步规则到Prometheus
// 接收完整的规则列表，生成Prometheus规则文件并重载配置
func (s *AlertService) SyncRulesToPrometheus(rules []model.AlertRule, ruleMetas []model.AlertRuleMeta) error {
	// 保存到内存缓存
	s.currentRules = rules
	s.currentRuleMetas = ruleMetas

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

		// 通过 alert_name 直接查找对应的规则模板
		// AlertRuleMeta.alert_name 关联 AlertRule.name
		rule = ruleMap[meta.AlertName]

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

		// 使用规则名作为 alert 名称，通过 labels 区分不同实例
		promRule := model.PrometheusRule{
			Alert:       rule.Name, // 使用规则名作为 alert 名称
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
		Msg("Prometheus rules file updated locally")

	// 同步到 Prometheus 容器
	if err := s.syncToPrometheusContainer(); err != nil {
		log.Warn().Err(err).Msg("Failed to sync rules to Prometheus container")
		// 不返回错误，因为本地文件已经生成成功
	}

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

// syncToPrometheusContainer 同步规则文件到本地 Prometheus 容器
func (s *AlertService) syncToPrometheusContainer() error {
	// 获取容器名称，默认为 mock-s3-prometheus
	containerName := os.Getenv("PROMETHEUS_CONTAINER")
	if containerName == "" {
		containerName = "mock-s3-prometheus"
	}

	// 1. 创建容器内的规则目录（如果不存在）
	cmdMkdir := exec.Command("docker", "exec", containerName, "mkdir", "-p", "/etc/prometheus/rules")
	if output, err := cmdMkdir.CombinedOutput(); err != nil {
		// 目录可能已存在，记录日志但不返回错误
		log.Debug().
			Str("output", string(output)).
			Msg("mkdir in container (may already exist)")
	}

	// 2. 将规则文件拷贝到容器内
	cmdCopy := exec.Command("docker", "cp", s.rulesFilePath, fmt.Sprintf("%s:/etc/prometheus/rules/alert_rules.yml", containerName))
	if output, err := cmdCopy.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy rules file to container: %w, output: %s", err, string(output))
	}

	log.Info().
		Str("container", containerName).
		Str("file", s.rulesFilePath).
		Msg("Rules synced to Prometheus container")

	// 3. 确保 Prometheus 配置包含 rule_files
	if err := s.ensurePrometheusRuleConfig(containerName); err != nil {
		log.Warn().Err(err).Msg("Failed to ensure Prometheus rule configuration")
	}

	return nil
}

// ensurePrometheusRuleConfig 确保 Prometheus 配置文件包含 rule_files 配置
func (s *AlertService) ensurePrometheusRuleConfig(containerName string) error {
	configPath := "/etc/prometheus/prometheus.yml"

	// 1. 检查配置文件是否已包含 rule_files
	cmdCheck := exec.Command("docker", "exec", containerName, "grep", "-q", "rule_files:", configPath)
	if err := cmdCheck.Run(); err == nil {
		// 已经包含 rule_files，不需要修改
		log.Debug().Msg("Prometheus config already contains rule_files")
		return nil
	}

	log.Info().Msg("Adding rule_files configuration to Prometheus")

	// 3. 在 global 部分后添加 rule_files 配置
	// 使用 sed 在 global: 块后插入 rule_files 配置
	sedScript := `'/^global:/,/^[^[:space:]]/ {
		/^[^[:space:]]/ {
			i\
# Alert rules\
rule_files:\
  - "/etc/prometheus/rules/*.yml"\

		}
	}'`

	cmdSed := exec.Command("docker", "exec", containerName, "sh", "-c",
		fmt.Sprintf(`sed -i '%s' %s`, sedScript, configPath))

	if output, err := cmdSed.CombinedOutput(); err != nil {
		// 如果 sed 失败，尝试使用更简单的方法
		log.Warn().
			Str("output", string(output)).
			Msg("sed failed, trying alternative method")

		// 使用 awk 方法
		awkScript := `awk '/^global:/ {print; getline; print; print "# Alert rules"; print "rule_files:"; print "  - \"/etc/prometheus/rules/*.yml\""; next} {print}' %s > %s.tmp && mv %s.tmp %s`
		cmdAwk := exec.Command("docker", "exec", containerName, "sh", "-c",
			fmt.Sprintf(awkScript, configPath, configPath, configPath, configPath))

		if output, err := cmdAwk.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to add rule_files to config: %w, output: %s", err, string(output))
		}
	}

	log.Info().Msg("Successfully added rule_files configuration to Prometheus")

	// 4. 重启 Prometheus 容器以应用配置
	cmdRestart := exec.Command("docker", "restart", containerName)
	if output, err := cmdRestart.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restart Prometheus: %w, output: %s", err, string(output))
	}

	log.Info().Msg("Prometheus restarted with new configuration")
	return nil
}

// UpdateRule 更新单个规则模板
// 只更新传入的规则，其他规则和所有元信息保持不变
func (s *AlertService) UpdateRule(rule model.AlertRule) error {
	// 查找并更新规则
	found := false
	for i, r := range s.currentRules {
		if r.Name == rule.Name {
			s.currentRules[i] = rule
			found = true
			break
		}
	}

	if !found {
		// 如果规则不存在，添加新规则
		s.currentRules = append(s.currentRules, rule)
	}

	// 统计受影响的元信息数量
	affectedCount := 0
	for _, meta := range s.currentRuleMetas {
		if meta.AlertName == rule.Name {
			affectedCount++
		}
	}

	log.Info().
		Str("rule", rule.Name).
		Int("affected_metas", affectedCount).
		Msg("Updating rule and affected metas")

	// 使用更新后的规则重新生成并同步
	return s.regenerateAndSync()
}

// UpdateRuleMeta 更新单个规则元信息
// 通过 alert_name + labels 唯一确定一个元信息记录
func (s *AlertService) UpdateRuleMeta(meta model.AlertRuleMeta) error {
	// 查找并更新元信息
	found := false
	for i, m := range s.currentRuleMetas {
		// 通过 alert_name + labels 唯一确定
		if m.AlertName == meta.AlertName && m.Labels == meta.Labels {
			s.currentRuleMetas[i] = meta
			found = true
			break
		}
	}

	if !found {
		// 如果元信息不存在，添加新元信息
		s.currentRuleMetas = append(s.currentRuleMetas, meta)
	}

	log.Info().
		Str("alert_name", meta.AlertName).
		Str("labels", meta.Labels).
		Msg("Updating rule meta")

	// 使用更新后的元信息重新生成并同步
	return s.regenerateAndSync()
}

// regenerateAndSync 使用当前内存中的规则和元信息重新生成Prometheus规则并同步
func (s *AlertService) regenerateAndSync() error {
	// 构建Prometheus规则文件
	prometheusRules := s.buildPrometheusRules(s.currentRules, s.currentRuleMetas)

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
		Int("rules_count", len(s.currentRules)).
		Int("metas_count", len(s.currentRuleMetas)).
		Msg("Rules regenerated and synced to Prometheus")

	return nil
}

// GetAffectedMetas 获取受影响的元信息数量
func (s *AlertService) GetAffectedMetas(ruleName string) int {
	count := 0
	for _, meta := range s.currentRuleMetas {
		if meta.AlertName == ruleName {
			count++
		}
	}
	return count
}
