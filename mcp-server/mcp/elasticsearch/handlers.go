package elasticsearch

import (
	"context"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"qiniu1024-mcp-server/datasource/elasticsearch"
)

func ErrorQueryHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	host := req.GetString("host", " ")
	startTime := req.GetString("startTime", " ")
	endTime := req.GetString("endTime", " ")

	data, err := elasticsearch.FetchErrorInPeriod(host, startTime, endTime)

	if err != nil {
		fmt.Println("查询日志失败")
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{mcp.NewTextContent(err.Error())},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(data))},
	}, nil
}
