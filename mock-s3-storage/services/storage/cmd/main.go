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

	"storage-service/internal/config"
	"storage-service/internal/handlers"
	"storage-service/internal/middleware"
	"storage-service/internal/services"

	"shared/discovery"
	"shared/faults"
	"shared/httpserver"
	"shared/telemetry/metrics"
	"shared/telemetry/tracing"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Printf("加载配置失败，使用默认配置: %v", err)
		cfg = config.GetDefaultConfig()
	}

	log.Printf("启动存储服务: %s v%s", cfg.Service.Name, cfg.Service.Version)

	// 初始化组件
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建故障注入引擎
	injectionEngine := faults.NewInjectionEngine()

	// 创建存储服务
	storageService := services.NewFilesystemStorageService(cfg.Storage.BaseDir, cfg.Service.MaxFileSize, injectionEngine)

	// 初始化存储目录
	if err := storageService.Initialize(); err != nil {
		log.Fatalf("初始化存储目录失败: %v", err)
	}

	// 创建可观测性组件
	metricsCollector := metrics.NewMetrics(cfg.Metrics)
	tracer := tracing.NewTracer(cfg.Tracing)

	// 创建处理器
	filesHandler := handlers.NewFilesHandler(storageService)
	healthHandler := handlers.NewHealthHandler()

	// 创建基础路由
	router := handlers.NewRouter(filesHandler, healthHandler)

	// 应用中间件（注意顺序）
	var handler http.Handler = router

	// 1. 请求ID中间件
	handler = middleware.RequestID()(handler)

	// 2. CORS中间件
	handler = middleware.CORS()(handler)

	// 3. 链路追踪中间件
	handler = middleware.TracingMiddleware(tracer)(handler)

	// 4. 指标收集中间件
	handler = middleware.MetricsMiddleware(metricsCollector)(handler)

	// 5. 故障注入中间件（最后应用，这样可以影响所有后续处理）
	handler = middleware.FaultInjectionMiddleware(injectionEngine, cfg.Service.Name)(handler)

	// 创建HTTP服务器
	server := httpserver.NewServer(cfg.Server)
	server.AddHandler("/", handler)

	// 添加指标端点
	if metricsCollector != nil && cfg.Metrics.Enabled {
		metricsServer := &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Metrics.Port),
			Handler: metricsCollector.Handler(),
		}

		go func() {
			log.Printf("指标服务启动在端口 :%d%s", cfg.Metrics.Port, cfg.Metrics.Path)
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("指标服务启动失败: %v", err)
			}
		}()

		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			metricsServer.Shutdown(ctx)
		}()
	}

	// 服务发现注册（如果启用）
	var serviceRegistry discovery.ServiceRegistry
	if cfg.Discovery.Address != "" {
		serviceDiscovery, err := discovery.NewConsulDiscovery(cfg.Discovery)
		if err != nil {
			log.Printf("创建服务发现失败: %v", err)
		} else {
			serviceInfo := discovery.ServiceInfo{
				ID:      fmt.Sprintf("%s-%d", cfg.Service.Name, os.Getpid()),
				Name:    cfg.Service.Name,
				Address: cfg.Server.Host,
				Port:    cfg.Server.Port,
				Health:  true,
			}

			serviceRegistry = discovery.NewRegistry(serviceDiscovery, serviceInfo)
			if err := serviceRegistry.RegisterSelf(ctx, cfg.Service.Name, cfg.Server.Host, cfg.Server.Port); err != nil {
				log.Printf("服务注册失败: %v", err)
			} else {
				log.Printf("服务已注册到 %s", cfg.Discovery.Address)
			}

			defer func() {
				if err := serviceRegistry.DeregisterSelf(ctx); err != nil {
					log.Printf("服务注销失败: %v", err)
				}
			}()
		}
	}

	// 启动服务器
	go func() {
		log.Printf("存储服务启动在 %s", server.GetAddr())
		log.Printf("服务配置:")
		log.Printf("  - 服务名称: %s", cfg.Service.Name)
		log.Printf("  - 服务版本: %s", cfg.Service.Version)
		log.Printf("  - 存储类型: %s", cfg.Storage.Type)
		log.Printf("  - 存储目录: %s", cfg.Storage.BaseDir)
		log.Printf("  - 最大文件大小: %d bytes", cfg.Service.MaxFileSize)
		log.Printf("API端点:")
		log.Printf("  - 健康检查: GET /api/health")
		log.Printf("  - 就绪检查: GET /api/ready")
		log.Printf("  - 存活检查: GET /api/live")
		log.Printf("  - 文件上传: POST /api/files/upload")
		log.Printf("  - 文件下载: GET /api/files/download/{fileID}")
		log.Printf("  - 文件删除: DELETE /api/files/{fileID}")
		log.Printf("  - 文件信息: GET /api/files/{fileID}/info")
		log.Printf("  - 文件列表: GET /api/files")

		if err := server.Start(); err != nil {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Stop(shutdownCtx); err != nil {
		log.Printf("服务器关闭失败: %v", err)
	}

	if err := storageService.Close(); err != nil {
		log.Printf("存储服务关闭失败: %v", err)
	}

	log.Println("服务器已关闭")
}
