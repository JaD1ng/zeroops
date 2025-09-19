package service

import "github.com/qiniu/zeroops/internal/deploy/model"

// DeployService 发布服务接口，负责发布和回滚操作的执行
type DeployService interface {
	// ExecuteDeployment 触发指定服务版本的发布操作
	ExecuteDeployment(params *model.DeployParams) (*model.OperationResult, error)

	// ExecuteRollback 对指定实例执行回滚操作，支持单实例或批量实例回滚
	ExecuteRollback(params *model.RollbackParams) (*model.OperationResult, error)
}
