package prometheus

import (
	"encoding/json"
	"fmt"
	"time"
)

// PrometheusDataFormatter Prometheus数据格式化器
// 用于将Prometheus查询结果转换为格式化的JSON格式
type PrometheusDataFormatter struct {
	Timezone string // 时区设置，默认为Asia/Shanghai
}

// NewPrometheusDataFormatter 创建新的数据格式化器
func NewPrometheusDataFormatter(timezone string) *PrometheusDataFormatter {
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	return &PrometheusDataFormatter{
		Timezone: timezone,
	}
}

// FormattedTimeData 格式化后的时间数据结构
type FormattedTimeData struct {
	Timestamp     string                 `json:"timestamp"`      // 格式化的时间戳
	TimestampUnix float64                `json:"timestamp_unix"` // 原始Unix时间戳
	Value         interface{}            `json:"value"`          // 数据值
	Metric        map[string]string      `json:"metric"`         // 指标标签
	Metadata      map[string]interface{} `json:"metadata"`       // 元数据信息
}

// PrometheusResponse Prometheus查询响应结构
type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string        `json:"resultType"`
		Result     []interface{} `json:"result"`
	} `json:"data"`
}

// FormatTimestampWithTimezone 将Unix时间戳转换为带时区的完整时间格式
// 参数说明：
// - timestamp: Unix时间戳（秒级，可以是浮点数）
// - timezone: 时区字符串，如"Asia/Shanghai"
// 返回值：
// - 格式化的时间字符串，如"2025-01-14T12:55:16+08:00"
// - 错误信息
func (f *PrometheusDataFormatter) FormatTimestampWithTimezone(timestamp float64, timezone string) (string, error) {
	// 如果未指定时区，使用默认时区
	if timezone == "" {
		timezone = f.Timezone
	}

	// 加载时区
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return "", fmt.Errorf("加载时区失败: %w", err)
	}

	// 将时间戳转换为time.Time对象
	// 分离整数部分和小数部分
	seconds := int64(timestamp)
	nanoseconds := int64((timestamp - float64(seconds)) * 1e9)

	t := time.Unix(seconds, nanoseconds).In(loc)

	// 格式化为ISO 8601格式，包含时区偏移
	return t.Format("2006-01-02T15:04:05-07:00"), nil
}

// FormatPrometheusData 格式化Prometheus查询结果
// 参数说明：
// - rawData: Prometheus原始查询结果（JSON字符串）
// - includeMetadata: 是否包含元数据信息
// 返回值：
// - 格式化后的JSON字符串
// - 错误信息
func (f *PrometheusDataFormatter) FormatPrometheusData(rawData string, includeMetadata bool) (string, error) {
	// 首先尝试解析为标准Prometheus响应格式
	var response PrometheusResponse
	if err := json.Unmarshal([]byte(rawData), &response); err != nil {
		// 如果标准格式解析失败，尝试解析为数组格式（兼容旧版本）
		var arrayResult []interface{}
		if arrayErr := json.Unmarshal([]byte(rawData), &arrayResult); arrayErr != nil {
			return "", fmt.Errorf("解析Prometheus响应失败: %w", err)
		}

		// 构建标准格式的响应
		response = PrometheusResponse{
			Status: "success",
		}
		response.Data.ResultType = "vector" // 默认为vector类型
		response.Data.Result = arrayResult
	}

	// 检查响应状态
	if response.Status != "success" {
		return "", fmt.Errorf("Prometheus查询失败，状态: %s", response.Status)
	}

	// 处理不同类型的查询结果
	var formattedResults []FormattedTimeData

	switch response.Data.ResultType {
	case "vector":
		// 处理瞬时向量查询结果
		formattedResults = f.formatVectorResult(response.Data.Result, includeMetadata)
	case "matrix":
		// 处理范围向量查询结果
		formattedResults = f.formatMatrixResult(response.Data.Result, includeMetadata)
	case "scalar":
		// 处理标量查询结果
		formattedResults = f.formatScalarResult(response.Data.Result, includeMetadata)
	default:
		return "", fmt.Errorf("不支持的查询结果类型: %s", response.Data.ResultType)
	}

	// 创建最终的响应结构
	finalResponse := map[string]interface{}{
		"status":       "success",
		"result_type":  response.Data.ResultType,
		"result_count": len(formattedResults),
		"timezone":     f.Timezone,
		"formatted_at": time.Now().Format("2006-01-02T15:04:05-07:00"),
		"data":         formattedResults,
	}

	// 转换为格式化的JSON
	formattedJSON, err := json.MarshalIndent(finalResponse, "", "  ")
	if err != nil {
		return "", fmt.Errorf("生成格式化JSON失败: %w", err)
	}

	return string(formattedJSON), nil
}

// formatVectorResult 格式化瞬时向量查询结果
func (f *PrometheusDataFormatter) formatVectorResult(results []interface{}, includeMetadata bool) []FormattedTimeData {
	var formattedResults []FormattedTimeData

	for _, result := range results {
		if resultMap, ok := result.(map[string]interface{}); ok {
			formattedData := FormattedTimeData{
				Metric: make(map[string]string),
			}

			// 处理指标标签
			if metric, ok := resultMap["metric"].(map[string]interface{}); ok {
				for key, value := range metric {
					if strValue, ok := value.(string); ok {
						formattedData.Metric[key] = strValue
					}
				}
			}

			// 处理值
			if value, ok := resultMap["value"].([]interface{}); ok && len(value) >= 2 {
				if timestamp, ok := value[0].(float64); ok {
					// 格式化时间戳
					formattedTime, err := f.FormatTimestampWithTimezone(timestamp, "")
					if err != nil {
						formattedTime = fmt.Sprintf("时间格式化错误: %v", err)
					}

					formattedData.Timestamp = formattedTime
					formattedData.TimestampUnix = timestamp
					formattedData.Value = value[1]
				}
			}

			// 添加元数据（可选）
			if includeMetadata {
				formattedData.Metadata = map[string]interface{}{
					"result_type":  "vector",
					"processed_at": time.Now().Format("2006-01-02T15:04:05-07:00"),
				}
			}

			formattedResults = append(formattedResults, formattedData)
		}
	}

	return formattedResults
}

// formatMatrixResult 格式化范围向量查询结果
func (f *PrometheusDataFormatter) formatMatrixResult(results []interface{}, includeMetadata bool) []FormattedTimeData {
	var formattedResults []FormattedTimeData

	for _, result := range results {
		if resultMap, ok := result.(map[string]interface{}); ok {
			// 处理指标标签
			metric := make(map[string]string)
			if metricData, ok := resultMap["metric"].(map[string]interface{}); ok {
				for key, value := range metricData {
					if strValue, ok := value.(string); ok {
						metric[key] = strValue
					}
				}
			}

			// 处理时间序列数据
			if values, ok := resultMap["values"].([]interface{}); ok {
				for _, valuePoint := range values {
					if point, ok := valuePoint.([]interface{}); ok && len(point) >= 2 {
						formattedData := FormattedTimeData{
							Metric: metric,
						}

						if timestamp, ok := point[0].(float64); ok {
							// 格式化时间戳
							formattedTime, err := f.FormatTimestampWithTimezone(timestamp, "")
							if err != nil {
								formattedTime = fmt.Sprintf("时间格式化错误: %v", err)
							}

							formattedData.Timestamp = formattedTime
							formattedData.TimestampUnix = timestamp
							formattedData.Value = point[1]
						}

						// 添加元数据（可选）
						if includeMetadata {
							formattedData.Metadata = map[string]interface{}{
								"result_type":  "matrix",
								"processed_at": time.Now().Format("2006-01-02T15:04:05-07:00"),
							}
						}

						formattedResults = append(formattedResults, formattedData)
					}
				}
			}
		}
	}

	return formattedResults
}

// formatScalarResult 格式化标量查询结果
func (f *PrometheusDataFormatter) formatScalarResult(results []interface{}, includeMetadata bool) []FormattedTimeData {
	var formattedResults []FormattedTimeData

	if len(results) > 0 {
		if scalar, ok := results[0].([]interface{}); ok && len(scalar) >= 2 {
			formattedData := FormattedTimeData{
				Metric: make(map[string]string),
			}

			if timestamp, ok := scalar[0].(float64); ok {
				// 格式化时间戳
				formattedTime, err := f.FormatTimestampWithTimezone(timestamp, "")
				if err != nil {
					formattedTime = fmt.Sprintf("时间格式化错误: %v", err)
				}

				formattedData.Timestamp = formattedTime
				formattedData.TimestampUnix = timestamp
				formattedData.Value = scalar[1]
			}

			// 添加元数据（可选）
			if includeMetadata {
				formattedData.Metadata = map[string]interface{}{
					"result_type":  "scalar",
					"processed_at": time.Now().Format("2006-01-02T15:04:05-07:00"),
				}
			}

			formattedResults = append(formattedResults, formattedData)
		}
	}

	return formattedResults
}

// FormatSimpleTimestamp 简单的时间戳格式化方法（向后兼容）
// 参数说明：
// - timestamp: Unix时间戳
// 返回值：
// - 格式化的时间字符串
func FormatSimpleTimestamp(timestamp float64) string {
	formatter := NewPrometheusDataFormatter("")
	formattedTime, err := formatter.FormatTimestampWithTimezone(timestamp, "")
	if err != nil {
		return fmt.Sprintf("时间格式化错误: %v", err)
	}
	return formattedTime
}

// FormatAndPrettyPrint 格式化数据并美化输出
// 参数说明：
// - rawData: 原始数据
// - timezone: 时区设置
// - includeMetadata: 是否包含元数据
// 返回值：
// - 美化后的JSON字符串
// - 错误信息
func FormatAndPrettyPrint(rawData string, timezone string, includeMetadata bool) (string, error) {
	formatter := NewPrometheusDataFormatter(timezone)
	return formatter.FormatPrometheusData(rawData, includeMetadata)
}
