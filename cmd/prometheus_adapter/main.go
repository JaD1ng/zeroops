package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 创建一个用于优雅关闭的context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 在goroutine中启动服务器
	serverErr := make(chan error, 1)
	go func() {
		log.Info().Msgf("Starting Prometheus Adapter on %s", cfg.Server.BindAddr)
		if err := router.Run(cfg.Server.BindAddr); err != nil {
			serverErr <- err
		}
	}()

	// 等待信号或服务器错误
	select {
	case sig := <-sigChan:
		log.Info().Msgf("Received signal %s, shutting down...", sig)

		// 创建超时context
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
		defer shutdownCancel()

		// 调用adapter的Shutdown方法
		if err := adapter.Close(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Error during shutdown")
		}

		log.Info().Msg("Shutdown complete")

	case err := <-serverErr:
		log.Fatal().Err(err).Msg("Server error")
	}
}
