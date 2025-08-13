package middleware

import (
	"context"
	"math/rand"
	"net/http"
	"shared/faults"
	"time"
)

// FaultInjectionMiddleware 故障注入中间件
func FaultInjectionMiddleware(engine faults.InjectionEngine, serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			endpoint := r.URL.Path
			
			// 检查是否应该注入故障
			if engine.ShouldInject(r.Context(), serviceName, endpoint) {
				rule := engine.GetInjectionRule(serviceName, endpoint)
				if rule != nil && rule.Enabled {
					// 根据注入率决定是否执行故障注入
					if rand.Float64() <= rule.Rate {
						if err := injectFault(w, r, rule, engine); err != nil {
							// 故障注入失败，继续正常处理
							next.ServeHTTP(w, r)
							return
						}
						return // 故障已注入，不继续处理
					}
				}
			}
			
			// 正常处理请求
			next.ServeHTTP(w, r)
		})
	}
}

// injectFault 执行具体的故障注入
func injectFault(w http.ResponseWriter, r *http.Request, rule *faults.InjectionRule, engine faults.InjectionEngine) error {
	switch rule.Type {
	case faults.InjectionTypeHTTPError:
		return injectHTTPError(w, rule, engine)
	case faults.InjectionTypeHTTPLatency:
		return injectHTTPLatency(w, r, rule)
	case faults.InjectionTypeGoroutineLeak:
		return injectGoroutineLeak(w, r, rule, engine)
	default:
		return nil
	}
}

// injectHTTPError 注入HTTP错误
func injectHTTPError(w http.ResponseWriter, rule *faults.InjectionRule, engine faults.InjectionEngine) error {
	appErr := engine.CreateError(rule)
	if appErr == nil {
		return nil
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.HTTPStatusCode())
	
	response := map[string]any{
		"success":    false,
		"error":      appErr.Error(),
		"error_type": string(appErr.Type),
		"error_code": appErr.Code,
		"request_id": appErr.RequestID,
		"injected":   true, // 标识这是故障注入的错误
	}
	
	return writeJSONResponse(w, response)
}

// injectHTTPLatency 注入HTTP延迟
func injectHTTPLatency(w http.ResponseWriter, r *http.Request, rule *faults.InjectionRule) error {
	// 从配置中获取延迟时间
	delayMs, ok := rule.Config["delay_ms"]
	if !ok {
		delayMs = 1000 // 默认1秒延迟
	}
	
	delay := time.Duration(delayMs.(float64)) * time.Millisecond
	
	// 创建带取消的上下文
	ctx, cancel := context.WithTimeout(r.Context(), delay+5*time.Second)
	defer cancel()
	
	select {
	case <-time.After(delay):
		// 延迟时间到，返回超时错误
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestTimeout)
		
		response := map[string]any{
			"success":    false,
			"error":      "Request timeout due to fault injection",
			"error_type": string(faults.ErrorTypeTimeout),
			"injected":   true,
		}
		
		return writeJSONResponse(w, response)
		
	case <-ctx.Done():
		// 上下文取消
		return ctx.Err()
	}
}

// injectGoroutineLeak 注入Goroutine泄漏
func injectGoroutineLeak(w http.ResponseWriter, _ *http.Request, rule *faults.InjectionRule, _ faults.InjectionEngine) error {
	// 从配置中获取泄漏类型和参数
	leakType, ok := rule.Config["leak_type"]
	if !ok {
		leakType = "infinite_loop" // 默认无限循环类型
	}
	
	leakCount, ok := rule.Config["leak_count"]
	if !ok {
		leakCount = 1.0 // 默认创建1个泄漏的goroutine
	}
	
	count := int(leakCount.(float64))
	
	// 创建泄漏的goroutine
	for range count {
		switch leakType {
		case "infinite_loop":
			// 创建无限循环的goroutine（消耗CPU）
			go func() {
				for {
					// 无限循环，不会退出
					time.Sleep(10 * time.Millisecond) // 轻微睡眠避免100% CPU
				}
			}()
			
		case "blocking_channel":
			// 创建阻塞在channel上的goroutine
			go func() {
				ch := make(chan struct{})
				// 永远阻塞等待channel
				<-ch
			}()
			
		case "infinite_select":
			// 创建在select中无限等待的goroutine
			go func() {
				ch := make(chan struct{})
				// 永远阻塞等待channel
				<-ch
			}()
			
		case "memory_growth":
			// 创建不断增长内存的goroutine
			go func() {
				var data [][]byte
				for {
					// 不断分配内存，模拟内存泄漏
					data = append(data, make([]byte, 1024*1024)) // 每次分配1MB
					time.Sleep(100 * time.Millisecond)
				}
			}()
		}
	}
	
	// 返回成功响应，但实际上已经创建了泄漏的goroutine
	appErr := &faults.AppError{
		Type:    faults.ErrorTypeGoroutineLeak,
		Code:    "GOROUTINE_LEAK_INJECTED",
		Message: "Goroutine leak has been injected",
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	
	response := map[string]any{
		"success":     false,
		"error":       appErr.Error(),
		"error_type":  string(appErr.Type),
		"error_code":  appErr.Code,
		"injected":    true,
		"leak_type":   leakType,
		"leak_count":  count,
		"description": "Goroutine泄漏已注入，检查系统资源使用情况",
	}
	
	return writeJSONResponse(w, response)
}