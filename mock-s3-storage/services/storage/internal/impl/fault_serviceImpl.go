package impl

import (
	"shared/faults"
	"shared/faults/cpu"
	"shared/faults/memory"
	"storage-service/internal/service"
	"time"
)

type faultServiceImpl struct {
	manager *faults.FaultManager
}

func NewFaultServiceImpl() service.FaultService {
	m := faults.NewFaultManager()
	// 注册所有故障实例
	m.Register(memory.NewMemLeakFault(1*1024*1024, 15000*time.Millisecond))
	m.Register(cpu.NewCpuSpikeFault(80, 4, 100*time.Millisecond)) // CPU使用率80%，4个工作goroutine，100ms间隔
	return &faultServiceImpl{
		manager: m,
	}
}

func (f *faultServiceImpl) StartFault(name string) error {
	return f.manager.Start(name)
}

func (f *faultServiceImpl) StopFault(name string) error {
	return f.manager.Stop(name)
}

func (f *faultServiceImpl) GetFaultStatus(name string) (string, error) {
	return f.manager.Status(name)
}

func (f *faultServiceImpl) ListFaults() ([]string, error) {
	return f.manager.List()
}
