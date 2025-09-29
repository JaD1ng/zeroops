package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/qiniu/zeroops/internal/prometheus_adapter/client"
	promconfig "github.com/qiniu/zeroops/internal/prometheus_adapter/config"
	"github.com/qiniu/zeroops/internal/prometheus_adapter/model"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// AlertService 告警服务 - 仅负责与Prometheus交互，不存储规则
type AlertService struct {
	promClient *client.PrometheusClient
	config     *promconfig.PrometheusAdapterConfig
	// 内存中缓存当前规则，用于增量更新
	currentRules     []model.AlertRule
	currentRuleMetas []model.AlertRuleMeta
	// 本地规则文件路径
	localRulesPath string
}

// NewAlertService 创建告警服务
func NewAlertService(promClient *client.PrometheusClient, config *promconfig.PrometheusAdapterConfig) *AlertService {
	service := &AlertService{
		promClient:       promClient,
		config:           config,
		currentRules:     []model.AlertRule{},
		currentRuleMetas: []model.AlertRuleMeta{},
		localRulesPath:   config.AlertRules.LocalFile,
	}

	// 启动时尝试加载本地规则
	if err := service.LoadRulesFromFile(); err != nil {
		log.Warn().Err(err).Msg("Failed to load rules from file, starting with empty rules")
	}

	return service
}

// ========== 持久化方法 ==========

// LoadRulesFromFile 从本地文件加载规则
func (s *AlertService) LoadRulesFromFile() error {
	// 检查文件是否存在
	if _, err := os.Stat(s.localRulesPath); os.IsNotExist(err) {
		log.Info().Str("path", s.localRulesPath).Msg("Local rules file does not exist, skipping load")
		return nil
	}

	// 读取文件内容
	data, err := os.ReadFile(s.localRulesPath)
	if err != nil {
		return fmt.Errorf("failed to read local rules file: %w", err)
	}

	// 解析规则文件
	var rulesFile model.PrometheusRuleFile
	if err := yaml.Unmarshal(data, &rulesFile); err != nil {
		return fmt.Errorf("failed to parse rules file: %w", err)
	}

	// 从Prometheus格式转换回内部格式
	s.currentRules = []model.AlertRule{}
	s.currentRuleMetas = []model.AlertRuleMeta{}

	// 用于去重的map
	ruleMap := make(map[string]*model.AlertRule)

	for _, group := range rulesFile.Groups {
		for _, rule := range group.Rules {
			// 提取基础规则信息
			ruleName := rule.Alert

			// 从annotations中获取description
			description := ""
			if desc, ok := rule.Annotations["description"]; ok {
				description = desc
			}

			// 从labels中获取severity
			severity := "warning"
			if sev, ok := rule.Labels["severity"]; ok {
				severity = sev
				delete(rule.Labels, "severity") // 移除severity，剩下的是meta的labels
			}

			// 创建或更新规则模板
			if _, exists := ruleMap[ruleName]; !exists {
				alertRule := model.AlertRule{
					Name:        ruleName,
					Description: description,
					Expr:        rule.Expr,
					Severity:    severity,
				}

				// 解析For字段获取WatchTime
				if rule.For != "" {
					// 简单解析，假设格式为 "300s" 或 "5m"
					if strings.HasSuffix(rule.For, "s") {
						if seconds, err := strconv.Atoi(strings.TrimSuffix(rule.For, "s")); err == nil {
							alertRule.WatchTime = seconds
						}
					} else if strings.HasSuffix(rule.For, "m") {
						if minutes, err := strconv.Atoi(strings.TrimSuffix(rule.For, "m")); err == nil {
							alertRule.WatchTime = minutes * 60
						}
					}
				}

				ruleMap[ruleName] = &alertRule
				s.currentRules = append(s.currentRules, alertRule)
			}

			// 创建元信息
			if len(rule.Labels) > 0 {
				labelsJSON, _ := json.Marshal(rule.Labels)
				meta := model.AlertRuleMeta{
					AlertName: ruleName,
					Labels:    string(labelsJSON),
				}

				// 从表达式中提取threshold（简单实现）
				// 假设表达式类似 "metric > 80" 或 "metric{labels} > 80"
				parts := strings.Split(rule.Expr, " ")
				if len(parts) >= 3 {
					if threshold, err := strconv.ParseFloat(parts[len(parts)-1], 64); err == nil {
						meta.Threshold = threshold
					}
				}

				s.currentRuleMetas = append(s.currentRuleMetas, meta)
			}
		}
	}

	log.Info().
		Int("rules", len(s.currentRules)).
		Int("metas", len(s.currentRuleMetas)).
		Str("path", s.localRulesPath).
		Msg("Loaded rules from local file")

	return nil
}

// SaveRulesToFile 保存规则到本地文件
func (s *AlertService) SaveRulesToFile() error {
	// 确保目录存在
	dir := filepath.Dir(s.localRulesPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create rules directory: %w", err)
	}

	// 构建Prometheus规则文件格式
	prometheusRules := s.buildPrometheusRules(s.currentRules, s.currentRuleMetas)

	// 序列化为YAML
	data, err := yaml.Marshal(prometheusRules)
	if err != nil {
		return fmt.Errorf("failed to marshal rules: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(s.localRulesPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write rules file: %w", err)
	}

	log.Info().
		Int("rules", len(s.currentRules)).
		Int("metas", len(s.currentRuleMetas)).
		Str("path", s.localRulesPath).
		Msg("Saved rules to local file")

	return nil
}

// Shutdown 优雅关闭，保存当前规则
func (s *AlertService) Shutdown() error {
	log.Info().Msg("Shutting down alert service, saving rules...")
	return s.SaveRulesToFile()
}

// ========== 公开 API 方法 ==========

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

// DeleteRule 删除单个规则模板及其所有关联的元信息
func (s *AlertService) DeleteRule(ruleName string) error {
	// 查找并删除规则模板
	ruleFound := false
	for i, rule := range s.currentRules {
		if rule.Name == ruleName {
			// 从切片中删除规则
			s.currentRules = append(s.currentRules[:i], s.currentRules[i+1:]...)
			ruleFound = true
			break
		}
	}

	if !ruleFound {
		return fmt.Errorf("rule '%s' not found", ruleName)
	}

	// 删除所有关联的元信息
	deletedMetaCount := 0
	newMetas := []model.AlertRuleMeta{}
	for _, meta := range s.currentRuleMetas {
		if meta.AlertName != ruleName {
			newMetas = append(newMetas, meta)
		} else {
			deletedMetaCount++
		}
	}
	s.currentRuleMetas = newMetas

	log.Info().
		Str("rule", ruleName).
		Int("deleted_metas", deletedMetaCount).
		Msg("Rule and associated metas deleted")

	// 重新生成并同步
	return s.regenerateAndSync()
}

// DeleteRuleMeta 删除单个规则元信息
func (s *AlertService) DeleteRuleMeta(ruleName, labels string) error {
	// 查找并删除匹配的元信息
	found := false
	for i, meta := range s.currentRuleMetas {
		if meta.AlertName == ruleName && meta.Labels == labels {
			// 从切片中删除元信息
			s.currentRuleMetas = append(s.currentRuleMetas[:i], s.currentRuleMetas[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("rule meta not found for rule '%s' with labels '%s'", ruleName, labels)
	}

	log.Info().
		Str("rule", ruleName).
		Str("labels", labels).
		Msg("Rule meta deleted")

	// 重新生成并同步
	return s.regenerateAndSync()
}

// ========== 内部核心方法 ==========

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

// ========== 规则构建相关方法 ==========

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
		// 通过 alert_name 直接查找对应的规则模板
		// AlertRuleMeta.alert_name 关联 AlertRule.name
		var rule *model.AlertRule = ruleMap[meta.AlertName]

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
			"summary":     fmt.Sprintf("%s %s %g", rule.Expr, rule.Op, meta.Threshold),
		}

		// 计算for字段
		forDuration := ""
		if rule.WatchTime > 0 {
			forDuration = fmt.Sprintf("%ds", rule.WatchTime)
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
				// 查找第一个 { 后的内容
				start := strings.Index(expr, "{")
				end := strings.Index(expr[start:], "}")
				if end != -1 {
					end += start
					existingLabels := strings.TrimSpace(expr[start+1 : end])
					if existingLabels == "" {
						// 空的标签选择器，直接替换
						expr = expr[:start+1] + strings.Join(labelMatchers, ",") + expr[end:]
					} else {
						// 已有标签，需要检查是否重复
						existingLabelMap := make(map[string]bool)
						// 解析现有标签
						labelPairs := strings.Split(existingLabels, ",")
						for _, pair := range labelPairs {
							if strings.Contains(pair, "=") {
								key := strings.TrimSpace(strings.Split(pair, "=")[0])
								if key != "" {
									existingLabelMap[key] = true
								}
							}
						}
						// 只添加不重复的标签
						newLabels := []string{}
						for k, v := range labels {
							if !existingLabelMap[k] && k != "" && v != "" {
								newLabels = append(newLabels, fmt.Sprintf(`%s="%s"`, k, v))
							}
						}
						if len(newLabels) > 0 {
							expr = expr[:end] + "," + strings.Join(newLabels, ",") + expr[end:]
						}
					}
				}
			} else {
				// 对于没有标签的简单指标，只处理单个单词的情况
				// 如果表达式包含空格、括号等，不进行标签注入
				if !strings.ContainsAny(expr, " ()[]{}") {
					// 只有单个指标名，可以安全添加标签
					expr = expr + "{" + strings.Join(labelMatchers, ",") + "}"
				}
			}
		}
	}

	// 添加比较操作符和阈值
	if meta.Threshold != 0 {
		expr = fmt.Sprintf("%s %s %g", expr, rule.Op, meta.Threshold)
	}

	return expr
}

// ========== 文件操作相关方法 ==========

// writeRulesFile 写入规则文件
func (s *AlertService) writeRulesFile(rules *model.PrometheusRuleFile) error {
	// 序列化为YAML
	data, err := yaml.Marshal(rules)
	if err != nil {
		return fmt.Errorf("failed to marshal rules: %w", err)
	}

	// 获取容器名称
	containerName := s.config.Prometheus.ContainerName

	// 直接写入到容器内的规则目录
	// 使用docker exec和echo命令写入文件
	cmd := exec.Command("docker", "exec", containerName, "sh", "-c",
		fmt.Sprintf("cat > /etc/prometheus/rules/alert_rules.yml << 'EOF'\n%s\nEOF", string(data)))

	if output, err := cmd.CombinedOutput(); err != nil {
		// 如果直接写入容器失败，尝试使用临时文件+docker cp
		log.Warn().
			Err(err).
			Str("output", string(output)).
			Msg("Failed to write directly to container, trying docker cp")

		// 写入临时文件
		tmpFile := "/tmp/prometheus_alert_rules.yml"
		if err := os.WriteFile(tmpFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write temp rules file: %w", err)
		}

		// 使用docker cp复制到容器
		if err := s.syncRuleFileToContainer(tmpFile); err != nil {
			return fmt.Errorf("failed to sync to container: %w", err)
		}

		// 清理临时文件
		os.Remove(tmpFile)
	}

	log.Info().
		Str("container", containerName).
		Int("groups", len(rules.Groups)).
		Msg("Prometheus rules file updated in container")

	return nil
}

// syncRuleFileToContainer 同步规则文件到容器
func (s *AlertService) syncRuleFileToContainer(filePath string) error {
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
	cmdCopy := exec.Command("docker", "cp", filePath, fmt.Sprintf("%s:/etc/prometheus/rules/alert_rules.yml", containerName))
	if output, err := cmdCopy.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy rules file to container: %w, output: %s", err, string(output))
	}

	log.Info().
		Str("container", containerName).
		Str("file", filePath).
		Msg("Rules synced to Prometheus container")

	// 3. 确保 Prometheus 配置包含 rule_files
	if err := s.ensurePrometheusRuleConfig(containerName); err != nil {
		log.Warn().Err(err).Msg("Failed to ensure Prometheus rule configuration")
	}

	return nil
}

// ========== Prometheus 配置相关方法 ==========

// reloadPrometheus 重新加载Prometheus配置
func (s *AlertService) reloadPrometheus() error {
	prometheusURL := s.config.Prometheus.Address

	reloadURL := fmt.Sprintf("%s/-/reload", strings.TrimSuffix(prometheusURL, "/"))

	resp, err := http.Post(reloadURL, "text/plain", nil)
	if err != nil {
		return fmt.Errorf("failed to reload Prometheus: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("prometheus reload failed with status: %d", resp.StatusCode)
	}

	log.Info().Msg("Prometheus configuration reloaded")
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
