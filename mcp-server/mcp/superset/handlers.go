package superset

import (
	"context"
	"fmt"
	"qiniu1024-mcp-server/config"
	"qiniu1024-mcp-server/datasource/superset"

	"github.com/mark3labs/mcp-go/mcp"
)

func SupersetQueryHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 规定参数对应的key和默认值
	sql := req.GetString("sql", "")
	schema := req.GetString("schema", "kodo")
	databaseID := req.GetInt("databaseID", 1)

	// 从全局配置GlobalConfig中读取superset的配置信息，创建superset的client结构体
	client := superset.NewSupersetClient(
		config.GlobalConfig.Superset.BaseURL,
		config.GlobalConfig.Superset.Username,
		config.GlobalConfig.Superset.Password,
	)
	if err := client.FetchCSRFToken(); err != nil || client.Login() != nil {
		return mcp.NewToolResultError("认证失败"), nil
	}

	// 调用具体实现方法获取数据
	data, err := client.FetchBySQL(sql, schema, databaseID)

	if err != nil {
		fmt.Println("获取superset数据失败")
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{mcp.NewTextContent(err.Error())},
		}, nil
	}

	// 添加数据处理逻辑

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(data))},
	}, nil
}
