package main

import (
	"context"
	"fmt"
	"log"
	"metadata/internal/config"
	"metadata/internal/handler"
	"metadata/internal/repository"
	"metadata/internal/service"
	"os"
	"os/signal"
	"shared/database"
	"shared/discovery"
	"shared/faults"
	"shared/faults/memory"
	"shared/httpserver"
	logs "shared/telemetry/logger"
	"shared/telemetry/metrics"
	"shared/telemetry/tracing"
	"syscall"
	"time"
)

func main() {
	configFile := os.Getenv("CONFIG_FILE")
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := logs.NewLogger(cfg.Logging)
	metrics := metrics.NewMetrics(cfg.Metrics)
	_ = tracing.NewTracer(cfg.Tracing)

	logger.Info(ctx, "启动metadata服务", map[string]any{
		"service":     cfg.Service.Name,
		"version":     cfg.Service.Version,
		"environment": cfg.Service.Environment,
	})

	db, err := database.NewSQL(cfg.Database)
	if err != nil {
		logger.Error(ctx, "连接数据库失败", err, nil)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Connect(ctx); err != nil {
		logger.Error(ctx, "数据库连接测试失败", err, nil)
		os.Exit(1)
	}

	cache, err := database.NewCache(cfg.Cache)
	if err != nil {
		logger.Error(ctx, "连接缓存失败", err, nil)
		os.Exit(1)
	}
	defer cache.Close()

	if err := cache.Connect(ctx); err != nil {
		logger.Error(ctx, "缓存连接测试失败", err, nil)
		os.Exit(1)
	}

	faultMgr := faults.NewFaultManager()
	memLeakFault := memory.NewMemLeakFault(1024*1024, 5*time.Second)
	faultMgr.Register(memLeakFault)

	metadataRepo := repository.NewMetadataRepository(db, cache)
	metadataService := service.NewMetadataService(metadataRepo, faultMgr, logger, metrics)
	metadataHandler := handler.NewMetadataHandler(metadataService, logger, metrics)

	server := httpserver.NewServer(cfg.HTTP)
	handler.RegisterRoutes(server, metadataHandler)

	serviceDiscovery, err := discovery.NewConsulDiscovery(cfg.Discovery)
	if err != nil {
		logger.Error(ctx, "创建服务发现客户端失败", err, nil)
		os.Exit(1)
	}
	defer serviceDiscovery.Close()

	serviceInfo := &discovery.ServiceInfo{
		ID:      fmt.Sprintf("%s-%d", cfg.Service.Name, os.Getpid()),
		Name:    cfg.Service.Name,
		Address: cfg.HTTP.Host,
		Port:    cfg.HTTP.Port,
		Health:  true,
	}

	if err := serviceDiscovery.Register(ctx, serviceInfo); err != nil {
		logger.Warn(ctx, "服务注册失败", map[string]any{"error": err.Error()})
	}

	go func() {
		logger.Info(ctx, "启动HTTP服务器", map[string]any{
			"address": fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port),
		})
		if err := server.Start(); err != nil {
			logger.Error(ctx, "HTTP服务器启动失败", err, nil)
			cancel()
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		logger.Info(ctx, "收到信号，开始关闭", map[string]any{"signal": sig.String()})
	case <-ctx.Done():
		logger.Info(ctx, "上下文取消，开始关闭", nil)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Service.ShutdownTimeout)
	defer shutdownCancel()

	if err := serviceDiscovery.Deregister(shutdownCtx, serviceInfo.ID); err != nil {
		logger.Warn(shutdownCtx, "服务注销失败", map[string]any{"error": err.Error()})
	}

	if err := server.Stop(shutdownCtx); err != nil {
		logger.Error(shutdownCtx, "HTTP服务器关闭失败", err, nil)
	}

	logger.Info(shutdownCtx, "metadata服务已停止", nil)
}
