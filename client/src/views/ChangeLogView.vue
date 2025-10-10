<template>
  <div class="changelog-container">
    <!-- 顶部导航 -->
    <div class="header">
      <div class="nav-section">
        <el-button @click="$router.push('/')" class="back-btn">
          <el-icon><ArrowLeft /></el-icon>
          返回首页
        </el-button>
        <div class="divider"></div>
        <div class="title">Zero Ops</div>
      </div>
    </div>

    <div class="page-title">系统状态变更记录</div>

    <!-- 标签页 -->
    <el-tabs v-model="activeTab" class="tabs">
      <el-tab-pane label="服务变更记录" name="service">
        <!-- 搜索框 -->
        <div class="search-section">
          <el-input
            v-model="searchKeyword"
            placeholder="搜索服务/版本/状态…"
            clearable
            @input="handleSearch"
          />
        </div>

        <!-- 变更记录列表 -->
        <div class="change-list">
          <!-- 加载状态 -->
          <div v-if="deploymentLoading" class="loading-container">
            <el-icon class="is-loading"><Loading /></el-icon>
            <span>加载服务变更记录中...</span>
          </div>
          <!-- 数据列表 -->
          <template v-else>
            <ChangeCard
              v-for="item in filteredChangeItems"
              :key="item.id"
              :item="item"
            />
            <div v-if="filteredChangeItems.length === 0" class="no-results">
              无匹配记录
            </div>
          </template>
        </div>
      </el-tab-pane>

      <el-tab-pane label="告警变更记录" name="alarm">
        <div class="change-list">
          <!-- 加载状态 -->
          <div v-if="alertRuleLoading" class="loading-container">
            <el-icon class="is-loading"><Loading /></el-icon>
            <span>加载告警变更记录中...</span>
          </div>
          <!-- 数据列表 -->
          <template v-else>
            <AlarmChangeCard
              v-for="item in alarmChangeItems"
              :key="item.id"
              :item="item"
            />
            <div v-if="alarmChangeItems.length === 0" class="no-results">
              暂无告警变更记录
            </div>
          </template>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useAppStore, type ChangeItem, type AlarmChangeItem } from '@/stores/app'
import { mockApi } from '@/mock/api'
import { apiService } from '@/api'
import type { DeploymentChangelogResponse } from '@/mock/services'
import ChangeCard from '@/components/ChangeCard.vue'
import AlarmChangeCard from '@/components/AlarmChangeCard.vue'
import { ArrowLeft, Loading } from '@element-plus/icons-vue'

interface AlertRuleChangeValue {
  name: string
  old: string
  new: string
}

interface AlertRuleChangeItem {
  name: string
  editTime: string
  scope: string
  values: AlertRuleChangeValue[]
  reason: string
}

interface AlertRuleChangelogResponse {
  items: AlertRuleChangeItem[]
  next?: string
}

const appStore = useAppStore()

const activeTab = ref('service')
const searchKeyword = ref('')
const deploymentLoading = ref(false)
const alertRuleLoading = ref(false)
const error = ref<string | null>(null)

// 部署变更记录数据
const deploymentChangelog = ref<DeploymentChangelogResponse | null>(null)
const changeItems = ref<ChangeItem[]>([])

// 告警规则变更记录数据
const alertRuleChangelog = ref<AlertRuleChangelogResponse | null>(null)
const alarmChangeItems = ref<AlarmChangeItem[]>([])

// 数据转换函数：将API返回的数据转换为前端需要的格式
const transformDeploymentChangelogToChangeItems = (changelogData: any[]): ChangeItem[] => {
  return changelogData.map((item, index) => {
    // 根据健康状态确定状态
    let state: '发布中' | '灰度中' | '已完成'
    let progress: number | undefined
    let ok: boolean
    
    if (item.endTime) {
      // 有结束时间，说明已完成
      state = '已完成'
      progress = 100
      ok = item.health === 'Normal'
    } else if (item.instances < item.totalInstances) {
      // 实例数小于总数，说明在灰度中
      state = '灰度中'
      progress = Math.round((item.instances / item.totalInstances) * 100)
      ok = item.health === 'Normal'
    } else {
      // 实例数等于总数，说明在发布中
      state = '发布中'
      progress = 50 // 默认进度
      ok = item.health === 'Normal'
    }
    
    // 如果有分批次数据，直接使用；否则创建默认批次
    let batches = item.batches || [
      {
        name: '部署批次',
        status: item.health === 'Normal' ? '正常' : item.health === 'Warning' ? '异常' : '异常',
        start: new Date(item.startTime).toLocaleString('zh-CN'),
        end: item.endTime ? new Date(item.endTime).toLocaleString('zh-CN') : '-',
        anomaly: item.health !== 'Normal' ? `${item.service}服务指标异常` : undefined
      }
    ]
    
    return {
      id: `chg-${item.service}-${item.version}-${item.startTime}`,
      service: item.service,
      version: item.version,
      state,
      progress,
      ok,
      batches
    }
  })
}

// 数据转换函数：将告警规则变更记录API返回的数据转换为前端需要的格式
const transformAlertRuleChangelogToAlarmChangeItems = (changelogData: AlertRuleChangeItem[]): AlarmChangeItem[] => {
  return changelogData.map((item, index) => {
    // 从scope中提取服务名和版本号
    let serviceName = '全局'
    if (item.scope?.startsWith('service:')) {
      const scopeValue = item.scope.slice('service:'.length) // 例如："metadata-servicev1.0.0"
      
      // 分离服务名和版本号（版本号通常以 v 开头后跟数字）
      const versionMatch = scopeValue.match(/v\d+\.\d+\.\d+$/)
      if (versionMatch) {
        const version = versionMatch[0]  // 例如："v1.0.0"
        const service = scopeValue.slice(0, -version.length)  // 去掉版本号部分
        serviceName = `${service} ${version}`  // 添加空格分隔，不带"服务"
      } else {
        serviceName = scopeValue  // 不带"服务"
      }
    }
    
    // 构建变更描述
    const changeDescription = item.values.map((value) => {
      return `${value.name}: ${value.old} -> ${value.new}`
    }).join(', ')
    
    // 格式化时间
    const timestamp = new Date(item.editTime).toLocaleString('zh-CN')
    
    return {
      id: `alarm-${item.name}-${item.editTime}`,
      service: serviceName,
      change: `${item.name}: ${changeDescription}`,
      timestamp,
      details: item.reason
    }
  })
}

// 加载部署变更记录
const loadDeploymentChangelog = async (start?: string, limit?: number) => {
  if (deploymentLoading.value) return // 防止重复加载
  
  try {
    deploymentLoading.value = true
    error.value = null
    
    const response = await apiService.getDeploymentChangelog(start, limit)
    deploymentChangelog.value = response.data
    
    // 转换数据格式
    changeItems.value = transformDeploymentChangelogToChangeItems(response.data.items)
    
    console.log('部署变更记录加载成功:', response.data)
  } catch (err) {
    error.value = '加载部署变更记录失败'
    console.error('加载部署变更记录失败:', err)
  } finally {
    deploymentLoading.value = false
  }
}

// 加载告警规则变更记录（使用真实 API）
const loadAlertRuleChangelog = async (start?: string, limit?: number) => {
  if (alertRuleLoading.value) return // 防止重复加载
  
  try {
    alertRuleLoading.value = true
    error.value = null
    
    const response = await apiService.getAlertRuleChangelog(start, limit ?? 10)
    alertRuleChangelog.value = response.data
    
    // 转换数据格式
    alarmChangeItems.value = transformAlertRuleChangelogToAlarmChangeItems(response.data.items)
    
    console.log('告警规则变更记录加载成功:', response.data)
  } catch (err) {
    error.value = '加载告警规则变更记录失败'
    console.error('加载告警规则变更记录失败:', err)
  } finally {
    alertRuleLoading.value = false
  }
}


// 计算属性
const filteredChangeItems = computed(() => {
  if (!searchKeyword.value) {
    return changeItems.value
  }
  
  const keyword = searchKeyword.value.toLowerCase()
  return changeItems.value.filter(item => 
    (item.service + item.version + item.state).toLowerCase().includes(keyword)
  )
})

// 方法
const handleSearch = () => {
  // 搜索逻辑已在计算属性中处理
}

// 监听标签页切换，实现按需加载
watch(activeTab, (newTab) => {
  if (newTab === 'service' && !changeItems.value.length) {
    loadDeploymentChangelog()
  } else if (newTab === 'alarm' && !alarmChangeItems.value.length) {
    loadAlertRuleChangelog()
  }
})

// 生命周期 - 只加载默认标签页数据
onMounted(() => {
  loadDeploymentChangelog() // 只加载默认标签页
})
</script>

<style scoped>
.changelog-container {
  height: 100vh;
  width: 100vw;
  background: linear-gradient(to bottom, #f8fafc, #ffffff);
  padding: 24px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.nav-section {
  display: flex;
  align-items: center;
  gap: 12px;
}

.back-btn {
  display: flex;
  align-items: center;
  gap: 4px;
}

.divider {
  width: 1px;
  height: 20px;
  background: #e2e8f0;
}

.title {
  font-size: 24px;
  font-weight: 600;
  color: #2c3e50;
}

.page-title {
  font-size: 32px;
  font-weight: 600;
  margin-bottom: 16px;
  color: #2c3e50;
}

.tabs {
  width: 100%;
}

.search-section {
  margin: 16px 0;
}

.change-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.no-results {
  text-align: center;
  color: #6b7280;
  font-size: 14px;
  padding: 32px;
}

.placeholder {
  text-align: center;
  color: #6b7280;
  font-size: 14px;
  padding: 32px;
}

.loading-container {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 32px;
  color: #6b7280;
  font-size: 14px;
}
</style>
