package superset

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// SqlQueryTool 根据sql语句从superset中获取数据
func SqlQueryTool(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool(
			"superset_query",
			mcp.WithDescription("查询Superset数据，通过SQL语句查询"),
			mcp.WithNumber("databaseID",
				mcp.Description("数据库ID，指向要拉取数据的数据库，本项目中绝大多数情况为1")),
			mcp.WithString("schema",
				mcp.Description("本项目中绝大多数情况为'kodo'")),
			mcp.WithString("sql",
				mcp.Required(),
				mcp.Description("sql语句，用于在Superset的特定表中查询数据")),
		),
		SupersetQueryHandler,
	)
}
