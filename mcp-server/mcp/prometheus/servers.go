package prometheus

import (
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/server"

	"qiniu1024-mcp-server/config"
)

func StartPrometheusMcpServer() {
	mcpServer := server.NewMCPServer(
		"Prometheus MCP Service",
		"1.0.0")

	// 添加工具
	mcpServer.AddTool(PromqlQueryTool(), PromqlQueryHandler)

	// 从配置文件读取端口号和 endpoint 路径
	port := config.GlobalConfig.Prometheus.Port
	endpoint := config.GlobalConfig.Prometheus.Endpoint
	httpServer := server.NewStreamableHTTPServer(mcpServer, server.WithEndpointPath(endpoint))
	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Prometheus MCP Service启动于 %s%s ...\n", addr, endpoint)
	log.Fatal(httpServer.Start(addr))
}
