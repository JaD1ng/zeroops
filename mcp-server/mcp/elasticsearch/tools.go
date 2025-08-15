package elasticsearch

import "github.com/mark3labs/mcp-go/mcp"

// ErrorQueryTool 根据宿主机id和给定时间段查询elasticsearch中的日志是否出现ERROR
func ErrorQueryTool() mcp.Tool {
	promqlQueryTool := mcp.NewTool(
		"elasticsearch_error_query",
		mcp.WithDescription("根据主机host的日志，判断给定时间段内该节点是否出现异常"),
		mcp.WithString("host",
			mcp.Required(),
			mcp.Description("宿主机或节点代码，确定要查哪一个节点的运行日志")),
		mcp.WithString("startTime",
			mcp.Required(),
			mcp.Description("查询时间段的开始时间")),
		mcp.WithString("endTime",
			mcp.Required(),
			mcp.Description("查询时间段的结束时间")),
	)
	return promqlQueryTool
}
