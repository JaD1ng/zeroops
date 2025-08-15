package elasticsearch

import (
	"fmt"
	"github.com/mark3labs/mcp-go/server"
	"log"
	"qiniu1024-mcp-server/config"
)

func StartPrometheusMcpServer() {
	mcpServer := server.NewMCPServer(
		"ElasticSearch MCP Service",
		"1.0.0")

	// 添加工具
	mcpServer.AddTool(ErrorQueryTool(), ErrorQueryHandler)
	// 从配置文件读取mcp-server运行的端口号和endpoint 路径
	port := config.GlobalConfig.ElasticSearch.Port
	endpoint := config.GlobalConfig.ElasticSearch.Endpoint
	httpServer := server.NewStreamableHTTPServer(mcpServer, server.WithEndpointPath(endpoint))
	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("ElasticSearch MCP Service启动于 %s%s ...\n", addr, endpoint)
	log.Fatal(httpServer.Start(addr))
}
