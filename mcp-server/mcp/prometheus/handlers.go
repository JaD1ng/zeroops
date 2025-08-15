package prometheus

import (
	"context"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"log"

	datahandler "qiniu1024-mcp-server/datahandler/prometheus"
	datasource "qiniu1024-mcp-server/datasource/prometheus"
)

// PromqlQueryHandler 处理PromQL查询请求
func PromqlQueryHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 获取请求参数
	regionCode := req.GetString("regionCode", "mock")
	promql := req.GetString("promql", "")

	// 参数验证
	if promql == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{mcp.NewTextContent("PromQL查询语句不能为空")},
		}, nil
	}

	log.Printf("开始执行PromQL查询: regionCode=%s, promql=%s", regionCode, promql)

	// 创建Prometheus客户端
	client, err := datasource.NewPrometheusClient(regionCode)
	if err != nil {
		log.Printf("创建Prometheus客户端失败: %v", err)
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("创建Prometheus客户端失败: %v", err))},
		}, nil
	}

	// 执行查询
	data, err := client.FetchByPromQl(promql)

	if err != nil {
		log.Printf("获取Prometheus数据失败: %v", err)
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("获取Prometheus数据失败: %v", err))},
		}, nil
	}

	log.Printf("成功获取原始数据，长度: %d", len(data))

	// 格式化数据
	formatter := datahandler.NewPrometheusDataFormatter("Asia/Shanghai")
	formattedData, err := formatter.FormatPrometheusData(data, true)

	if err != nil {
		log.Printf("格式化数据失败: %v", err)
		// 即使格式化失败，也返回原始数据
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("数据格式化失败，返回原始数据: %v\n\n原始数据:\n%s", err, data))},
		}, nil
	}
	//// 将获取的数据（json格式的字符串）写入文件
	//// 获取prometheus子目录的文件路径
	//prometheusDir := filepath.Join("prometheus", "test1.json")
	//filePath, err := config.GetFilePath(prometheusDir)
	//if err != nil {
	//	log.Printf("获取文件路径失败: %v", err)
	//} else {
	//	// 写入文件
	//	err = os.WriteFile(filePath, []byte(formattedData), 0644)
	//	if err != nil {
	//		log.Printf("文件写入失败: %v", err)
	//	} else {
	//		log.Printf("文件已保存到: %s", filePath)
	//	}
	//}

	log.Printf("数据格式化成功，返回格式化后的数据")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(formattedData)},
	}, nil
}
