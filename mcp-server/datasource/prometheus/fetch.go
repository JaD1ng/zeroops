package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"qiniu1024-mcp-server/config"
	"time"
)

// matchURL 根据 regionCode 返回对应的 Prometheus URL
func matchURL(regionCode string) (string, error) {
	// 根据regionCode全局配置 config.GlobalConfig 读取映射关系，获取url地址，提升灵活性和安全性
	url, ok := config.GlobalConfig.Prometheus.Regions[regionCode]
	if !ok {
		return "", fmt.Errorf("不支持的 regionCode: %s", regionCode)
	}
	return url, nil
}

// FetchByPromQl 执行PromQL查询并返回JSON格式的数据
func (p *PrometheusClient) FetchByPromQl(promql string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 执行PromQL查询
	result, warnings, err := p.Client.Query(ctx, promql, time.Now())
	if err != nil {
		return "", fmt.Errorf("PromQL 查询失败: %w", err)
	}

	// 记录警告信息
	if len(warnings) > 0 {
		fmt.Printf("Prometheus 查询警告: %v\n", warnings)
	}

	// 构建标准的Prometheus响应格式
	response := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"resultType": result.Type().String(),
			"result":     result,
		},
	}

	// 将结果转换为JSON格式
	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("JSON序列化失败: %w", err)
	}

	return string(jsonData), nil
}
