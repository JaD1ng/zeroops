package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"shared/faults"
)

// 演示如何使用Goroutine泄漏mock功能的示例

func main() {
	fmt.Println("=== Goroutine泄漏Mock示例 ===")
	
	// 示例1: HTTP中间件层面的Goroutine泄漏注入规则
	httpLeakRule := &faults.InjectionRule{
		ID:       "http-goroutine-leak-1",
		Type:     faults.InjectionTypeGoroutineLeak,
		Service:  "storage-service",
		Endpoint: "/api/files/upload",
		Rate:     1.0, // 100%触发
		Enabled:  true,
		Config: map[string]any{
			"leak_type":  "infinite_loop",    // 无限循环类型
			"leak_count": 2,                  // 创建2个泄漏的goroutine
		},
		ErrorType: faults.ErrorTypeGoroutineLeak,
		ErrorCode: "GOROUTINE_LEAK_INJECTED",
		ErrorMsg:  "HTTP层Goroutine泄漏已注入",
	}
	
	// 示例2: 存储服务层面的文件处理Goroutine泄漏规则
	storageLeakRule := &faults.InjectionRule{
		ID:       "storage-goroutine-leak-1",
		Type:     faults.InjectionTypeGoroutineLeak,
		Service:  "storage-service", 
		Endpoint: "/api/files/upload",
		Rate:     0.5, // 50%触发概率
		Enabled:  true,
		Config: map[string]any{
			"leak_type":  "file_processing",  // 文件处理类型
			"leak_count": 1,                  // 创建1个泄漏的goroutine
		},
		ErrorType: faults.ErrorTypeGoroutineLeak,
		ErrorCode: "UPLOAD_GOROUTINE_LEAK",
		ErrorMsg:  "文件上传过程中的Goroutine泄漏",
	}
	
	// 示例3: 元数据同步Goroutine泄漏规则
	metadataLeakRule := &faults.InjectionRule{
		ID:       "metadata-sync-leak-1",
		Type:     faults.InjectionTypeGoroutineLeak,
		Service:  "storage-service",
		Endpoint: "/api/files/upload",
		Rate:     0.3, // 30%触发概率
		Enabled:  true,
		Config: map[string]any{
			"leak_type":  "metadata_sync",    // 元数据同步类型
			"leak_count": 3,                  // 创建3个泄漏的goroutine
		},
		ErrorType: faults.ErrorTypeGoroutineLeak,
		ErrorCode: "METADATA_SYNC_LEAK", 
		ErrorMsg:  "元数据同步过程中的Goroutine泄漏",
	}
	
	// 打印规则配置
	printRule("HTTP层Goroutine泄漏规则", httpLeakRule)
	printRule("存储层文件处理泄漏规则", storageLeakRule)  
	printRule("元数据同步泄漏规则", metadataLeakRule)
	
	fmt.Println("\n=== 可用的泄漏类型 ===")
	fmt.Println("1. infinite_loop: 无限循环消耗CPU")
	fmt.Println("2. blocking_channel: 永远阻塞在channel上")
	fmt.Println("3. infinite_select: 在select中无限等待")
	fmt.Println("4. memory_growth: 不断增长内存")
	fmt.Println("5. file_processing: 文件扫描处理不停止")
	fmt.Println("6. metadata_sync: 元数据同步ticker不停止")
	fmt.Println("7. file_watcher: 文件监控循环不退出")
	
	fmt.Println("\n=== 使用方法 ===")
	fmt.Println("1. 通过故障注入引擎添加规则")
	fmt.Println("2. 发送HTTP请求到配置的endpoint")
	fmt.Println("3. 根据配置的概率触发Goroutine泄漏")
	fmt.Println("4. 使用 runtime.NumGoroutine() 监控泄漏情况")
	fmt.Println("5. 使用 pprof 分析goroutine堆栈")
	
	fmt.Println("\n=== 监控命令示例 ===")
	fmt.Println("# 查看当前goroutine数量")
	fmt.Println("curl http://localhost:8080/debug/pprof/goroutine?debug=1")
	fmt.Println("# 下载goroutine profile")
	fmt.Println("curl http://localhost:8080/debug/pprof/goroutine > goroutine.prof")
	fmt.Println("# 分析profile")
	fmt.Println("go tool pprof goroutine.prof")
}

func printRule(name string, rule *faults.InjectionRule) {
	fmt.Printf("\n=== %s ===\n", name)
	data, _ := json.MarshalIndent(rule, "", "  ")
	fmt.Println(string(data))
}

// 演示如何监控Goroutine数量的辅助函数
func MonitorGoroutines() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		// 这里可以添加实际的监控逻辑
		log.Printf("当前Goroutine数量监控点 - 时间: %v", time.Now().Format("15:04:05"))
	}
}