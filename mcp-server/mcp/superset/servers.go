package superset

import (
	"fmt"
	"log"

	"qiniu1024-mcp-server/config"

	"github.com/mark3labs/mcp-go/server"
)

func StartSupersetMcpServer() {
	s := server.NewMCPServer(
		"Superset MCP Service",
		"1.0.0")
	SqlQueryTool(s)

	// 从配置文件读取端口号和 endpoint 路径
	port := config.GlobalConfig.Superset.Port
	endpoint := config.GlobalConfig.Superset.Endpoint
	httpServer := server.NewStreamableHTTPServer(s, server.WithEndpointPath(endpoint))
	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Superset MCP Service启动于 %s%s ...\n", addr, endpoint)
	log.Fatal(httpServer.Start(addr))
}
