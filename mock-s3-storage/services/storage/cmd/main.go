package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	sharedconfig "shared/config"
	"shared/telemetry/metrics"
	"storage-service/internal/config"
	"storage-service/internal/handler"
	"storage-service/internal/repository"
)

const (
	// 优雅关闭超时时间
	gracefulShutdownTimeout = 30 * time.Second
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建存储工厂
	factory := repository.NewStorageFactory()

	// 创建PostgreSQL存储服务
	storageService, err := factory.CreatePostgresStorage(cfg.GetConnectionString(), cfg.Database.TableName)
	if err != nil {
		log.Fatalf("初始化存储服务失败: %v", err)
	}
	defer storageService.Close()

	// 创建文件处理器
	fileHandler := handler.NewFileHandler(storageService)

	// 创建故障处理器
	faultService := repository.NewFaultServiceImpl()
	faultHandler := handler.NewFaultHandler(faultService)

	// 创建指标收集器
	metricsConfig := sharedconfig.MetricsConfig{
		ServiceName: cfg.Metrics.ServiceName,
		ServiceVer:  cfg.Metrics.ServiceVer,
		Namespace:   cfg.Metrics.Namespace,
		Enabled:     cfg.Metrics.Enabled,
		Port:        cfg.Metrics.Port,
		Path:        cfg.Metrics.Path,
	}

	metricsCollector := metrics.NewMetrics(metricsConfig)
	defer metricsCollector.Close()

	// 创建路由处理器
	router := handler.NewRouter(fileHandler, faultHandler)

	// 创建HTTP服务器
	server := &http.Server{
		Addr:         cfg.GetServerAddr(),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// 启动服务器
	go func() {
		log.Printf("文件存储服务启动在 %s", cfg.GetServerAddr())
		log.Printf("数据库表名: %s", cfg.Database.TableName)
		log.Printf("API端点:")
		log.Printf("  - 健康检查: GET /api/health")
		log.Printf("  - 文件上传: POST /api/files/upload")
		log.Printf("  - 文件下载: GET /api/files/download/{fileID}")
		log.Printf("  - 文件删除: DELETE /api/files/{fileID}")
		log.Printf("  - 文件信息: GET /api/files/{fileID}/info")
		log.Printf("  - 文件列表: GET /api/files")

		log.Printf("  - 故障启动: POST /fault/start/{name}")
		log.Printf("  - 故障停止: POST /fault/stop/{name}")
		log.Printf("  - 故障状态: GET /fault/status/{name}")
		log.Printf("  - 故障列表: GET /fault/list")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	go func() {
		log.Printf("Prometheus metrics endpoint running at %s%s\n", cfg.GetMetricsAddr(), cfg.Metrics.Path)
		http.Handle("/metrics", metricsCollector.Handler())
		if err := http.ListenAndServe(cfg.GetMetricsAddr(), nil); err != nil {
			log.Fatalf("Prometheus metrics 服务启动失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")

	// 优雅关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("服务器关闭失败: %v", err)
	}

	log.Println("服务器已关闭")
}
