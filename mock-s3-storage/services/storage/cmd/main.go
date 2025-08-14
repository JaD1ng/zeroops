package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"shared/config"
	"shared/telemetry/metrics"
	"storage-service/internal/handler"
	"storage-service/internal/impl"
)

const (
	// 服务配置
	defaultPort = "8080"
	metricsPort = "1080"
	defaultHost = "127.0.0.1"

	// PostgreSQL配置
	dbHost     = "localhost"
	dbPort     = "5432"
	dbUser     = "postgres"
	dbPassword = "123456"
	dbName     = "mock"
	dbSSLMode  = "disable"

	// 表名配置
	defaultTableName = "files"
)

func main() {
	// 获取端口号（从环境变量或使用默认值）
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	// 获取表名（从环境变量或使用默认值）
	tableName := os.Getenv("TABLE_NAME")
	if tableName == "" {
		tableName = defaultTableName
	}

	// 构建数据库连接字符串
	connectionString := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPassword, dbName, dbSSLMode,
	)

	// 创建存储工厂
	factory := impl.NewStorageFactory()

	// 创建PostgreSQL存储服务
	storageService, err := factory.CreatePostgresStorage(connectionString, tableName)
	if err != nil {
		log.Fatalf("初始化存储服务失败: %v", err)
	}
	defer storageService.Close()

	// 创建文件处理器
	fileHandler := handler.NewFileHandler(storageService)

	// 创建故障处理器
	faultService := impl.NewFaultServiceImpl()
	faultHandler := handler.NewFaultHandler(faultService)

	// 创建指标收集器
	metricsConfig := config.MetricsConfig{
		ServiceName: "mock-storage",
		ServiceVer:  "1.0.0",
		Namespace:   "storage",
		Enabled:     true,
		Port:        9090,
		Path:        "/metrics",
	}

	metricsCollector := metrics.NewMetrics(metricsConfig)
	defer metricsCollector.Close()

	// 创建路由处理器
	router := handler.NewRouter(fileHandler, faultHandler)

	// 创建HTTP服务器
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", defaultHost, port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 启动服务器
	go func() {
		log.Printf("文件存储服务启动在 http://%s:%s", defaultHost, port)
		log.Printf("数据库表名: %s", tableName)
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
		log.Printf("Prometheus metrics endpoint running at :%s/metrics\n", metricsPort)
		http.Handle("/metrics", metricsCollector.Handler())
		if err := http.ListenAndServe(":"+metricsPort, nil); err != nil {
			log.Fatalf("Prometheus metrics 服务启动失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")

	// 优雅关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("服务器关闭失败: %v", err)
	}

	log.Println("服务器已关闭")
}
