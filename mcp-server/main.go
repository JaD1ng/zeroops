package main

import (
	"qiniu1024-mcp-server/config"
	"qiniu1024-mcp-server/mcp/elasticsearch"
	"qiniu1024-mcp-server/mcp/prometheus"
)

func main() {
	// 启动服务前，先加载配置文件 config.yaml
	if err := config.LoadConfig("config.yaml"); err != nil {
		panic("配置文件加载失败: " + err.Error())
	}

	go prometheus.StartPrometheusMcpServer()
	// go superset.StartSupersetMcpServer()
	go elasticsearch.StartPrometheusMcpServer()
	select {}
}
