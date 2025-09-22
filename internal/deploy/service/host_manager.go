package service

import (
	"github.com/qiniu/zeroops/internal/deploy/model"
)

// HostManager 主机管理接口，负责主机信息查询和管理
type HostManager interface {
	// GetHosts 获取发布系统管理的全部主机信息
	GetHosts() ([]*model.HostInfo, error)
}

// hostManager 主机管理实现
type hostManager struct {
	// TODO: 添加必要的字段
}

// NewHostManager 创建HostManager实例
func NewHostManager() HostManager {
	return &hostManager{
		// TODO: 初始化字段
	}
}

// GetHosts 实现获取全部主机信息
func (h *hostManager) GetHosts() ([]*model.HostInfo, error) {
	// TODO: 实现获取主机列表逻辑
	return nil, nil
}
