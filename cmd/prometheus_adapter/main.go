package main

import (
	"os"

	"github.com/fox-gonic/fox"
	"github.com/qiniu/zeroops/internal/config"
	prometheusadapter "github.com/qiniu/zeroops/internal/prometheus_adapter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// 配置日志
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msg("Starting Prometheus Adapter server")

	// 加载配置
	cfg := &config.Config{
		Server: config.ServerConfig{
			BindAddr: ":9999", // 默认端口
		},
	}

	// 如果有环境变量，使用环境变量的端口
	if port := os.Getenv("ADAPTER_PORT"); port != "" {
		cfg.Server.BindAddr = ":" + port
	}

	// 创建 Prometheus Adapter 服务器
	adapter, err := prometheusadapter.NewPrometheusAdapterServer(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Prometheus Adapter server")
	}

	// 创建路由
	router := fox.New()

	// 启动 API
	if err := adapter.UseApi(router); err != nil {
		log.Fatal().Err(err).Msg("Failed to setup API routes")
	}

	// 启动服务器
	log.Info().Msgf("Starting Prometheus Adapter on %s", cfg.Server.BindAddr)
	if err := router.Run(cfg.Server.BindAddr); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
