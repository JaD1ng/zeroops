package cpu

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"shared/faults"
)

// CpuSpikeFault CPU飙升故障
// 通过创建多个goroutine执行CPU密集型任务来模拟CPU使用率飙升
type CpuSpikeFault struct {
	running      int32         // 标志故障是否运行中
	stopCh       chan struct{} // 用于停止 goroutine
	stoppedWg    sync.WaitGroup
	cpuIntensity int           // CPU使用强度 (1-100)
	workerCount  int           // 工作goroutine数量
	workDelay    time.Duration // 工作间隔
}

// NewCpuSpikeFault 创建CPU飙升故障实例
// cpuIntensity: CPU使用强度，范围1-100，数值越大CPU使用率越高
// workerCount: 工作goroutine数量，建议设置为CPU核心数的1-4倍
// workDelay: 工作间隔，控制CPU使用的频率
func NewCpuSpikeFault(cpuIntensity int, workerCount int, workDelay time.Duration) *CpuSpikeFault {
	// 参数验证
	if cpuIntensity < 1 || cpuIntensity > 100 {
		cpuIntensity = 80 // 默认值
	}
	if workerCount < 1 {
		workerCount = runtime.NumCPU() // 默认使用CPU核心数
	}
	if workDelay < time.Millisecond {
		workDelay = 100 * time.Millisecond // 默认100ms
	}

	return &CpuSpikeFault{
		stopCh:       make(chan struct{}),
		cpuIntensity: cpuIntensity,
		workerCount:  workerCount,
		workDelay:    workDelay,
	}
}

// Name 返回故障名称
func (c *CpuSpikeFault) Name() string {
	return "CpuSpike"
}

// Start 启动CPU飙升故障
func (c *CpuSpikeFault) Start() error {
	// 首先判断运行状态，防止重复调用
	if !atomic.CompareAndSwapInt32(&c.running, 0, 1) {
		return fmt.Errorf("CpuSpikeFault already running")
	}
	c.stopCh = make(chan struct{})

	// 启动多个工作goroutine
	for i := 0; i < c.workerCount; i++ {
		c.stoppedWg.Add(1)
		go c.cpuWorker(i)
	}

	return nil
}

// cpuWorker CPU工作goroutine
// 执行CPU密集型任务来消耗CPU资源
func (c *CpuSpikeFault) cpuWorker(workerID int) {
	defer c.stoppedWg.Done()

	ticker := time.NewTicker(c.workDelay)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			// 根据CPU强度执行相应的工作量
			c.performCpuWork(c.cpuIntensity)
		}
	}
}

// performCpuWork 执行CPU密集型工作
// intensity: 工作强度，范围1-100
func (c *CpuSpikeFault) performCpuWork(intensity int) {
	// 根据强度计算工作循环次数
	cycles := intensity * 1000 // 基础循环次数
	if intensity > 50 {
		cycles = intensity * 2000 // 高强度时增加循环次数
	}

	// 执行CPU密集型计算
	for i := 0; i < cycles; i++ {
		// 数学计算，消耗CPU
		result := 0.0
		for j := 0; j < 100; j++ {
			result += float64(i) * float64(j) / float64(i+j+1)
		}
		_ = result // 避免编译器优化掉计算

		// 检查是否需要停止
		select {
		case <-c.stopCh:
			return
		default:
			// 继续执行
		}
	}
}

// Stop 停止CPU飙升故障
func (c *CpuSpikeFault) Stop() error {
	if !atomic.CompareAndSwapInt32(&c.running, 1, 0) {
		return fmt.Errorf("CpuSpikeFault not running")
	}
	close(c.stopCh)
	c.stoppedWg.Wait()
	return nil
}

// Status 返回故障状态
func (c *CpuSpikeFault) Status() string {
	if atomic.LoadInt32(&c.running) == 1 {
		return "running"
	}
	return "stopped"
}

// GetCpuIntensity 获取当前CPU使用强度
func (c *CpuSpikeFault) GetCpuIntensity() int {
	return c.cpuIntensity
}

// GetWorkerCount 获取当前工作goroutine数量
func (c *CpuSpikeFault) GetWorkerCount() int {
	return c.workerCount
}

// GetWorkDelay 获取当前工作间隔
func (c *CpuSpikeFault) GetWorkDelay() time.Duration {
	return c.workDelay
}

// 确保 CpuSpikeFault 实现了 faults.Fault 接口
var _ faults.Fault = (*CpuSpikeFault)(nil)
