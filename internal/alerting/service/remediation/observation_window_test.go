package remediation

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisObservationWindowManager(t *testing.T) {
	// 使用内存 Redis 客户端进行测试
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // 需要 Redis 实例
	})
	defer rdb.Close()

	// 检查 Redis 连接
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	manager := NewRedisObservationWindowManager(rdb)

	t.Run("StartObservation", func(t *testing.T) {
		service := "test-service"
		version := "v1.0.0"
		alertID := "test-alert-1"
		duration := 5 * time.Minute

		err := manager.StartObservation(ctx, service, version, alertID, duration)
		require.NoError(t, err)

		// 验证观察窗口已创建
		window, err := manager.CheckObservation(ctx, service, version)
		require.NoError(t, err)
		require.NotNil(t, window)
		assert.Equal(t, service, window.Service)
		assert.Equal(t, version, window.Version)
		assert.Equal(t, alertID, window.AlertID)
		assert.True(t, window.IsActive)
	})

	t.Run("CheckObservation_NotFound", func(t *testing.T) {
		service := "non-existent-service"
		version := "v1.0.0"

		window, err := manager.CheckObservation(ctx, service, version)
		require.NoError(t, err)
		assert.Nil(t, window)
	})

	t.Run("CompleteObservation", func(t *testing.T) {
		service := "test-service-2"
		version := "v1.0.0"
		alertID := "test-alert-2"
		duration := 5 * time.Minute

		// 先创建观察窗口
		err := manager.StartObservation(ctx, service, version, alertID, duration)
		require.NoError(t, err)

		// 完成观察窗口
		err = manager.CompleteObservation(ctx, service, version)
		require.NoError(t, err)

		// 验证观察窗口已被移除
		window, err := manager.CheckObservation(ctx, service, version)
		require.NoError(t, err)
		assert.Nil(t, window)
	})

	t.Run("CancelObservation", func(t *testing.T) {
		service := "test-service-3"
		version := "v1.0.0"
		alertID := "test-alert-3"
		duration := 5 * time.Minute

		// 先创建观察窗口
		err := manager.StartObservation(ctx, service, version, alertID, duration)
		require.NoError(t, err)

		// 取消观察窗口
		err = manager.CancelObservation(ctx, service, version)
		require.NoError(t, err)

		// 验证观察窗口已被移除
		window, err := manager.CheckObservation(ctx, service, version)
		require.NoError(t, err)
		assert.Nil(t, window)
	})
}

func TestGetObservationDuration(t *testing.T) {
	duration := GetObservationDuration()
	assert.Equal(t, 30*time.Minute, duration)
}
