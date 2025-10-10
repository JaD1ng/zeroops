<template>
  <div class="alerts-page">
    <div class="content-wrapper">
      <!-- 顶部导航 -->
      <div class="header">
        <div class="nav-section">
          <el-button 
            @click="$router.push('/')"
            class="back-button"
          >
            <el-icon><ArrowLeft /></el-icon>
            返回首页
          </el-button>
          <div class="divider"></div>
          <div class="title">Zero Ops</div>
        </div>
      </div>

      <!-- 页面标题 -->
      <div class="page-title">告警记录</div>

      <!-- 状态过滤 -->
      <div class="filter-section">
        <div class="filter-buttons">
          <button 
            v-for="filter in filters" 
            :key="filter.key"
            :class="['filter-btn', { active: filterState === filter.key }]"
            @click="setFilterState(filter.key as 'all' | 'open' | 'closed')"
          >
            <span>{{ filter.label }}</span>
            <span class="count-badge">{{ filter.count }}</span>
          </button>
        </div>
      </div>

      <!-- 加载状态 -->
      <div v-if="loading" class="loading-container">
        <el-icon class="is-loading"><Loading /></el-icon>
        <span>加载中...</span>
      </div>

      <!-- 错误状态 -->
      <div v-else-if="error" class="error-container">
        <el-icon><Warning /></el-icon>
        <span>{{ error }}</span>
        <el-button @click="loadAlerts" size="small">重试</el-button>
      </div>

      <!-- 告警列表 -->
      <div v-else class="alerts-list">
        <div 
          v-for="alert in alerts" 
          :key="alert.id"
          class="alert-card"
        >
          <div class="alert-content">
            <!-- 状态图标 -->
            <div class="status-icon">
              <el-icon v-if="alert.state === 'Open'" class="open-icon">
                <Warning />
              </el-icon>
              <el-icon v-else class="closed-icon">
                <Check />
              </el-icon>
            </div>

            <!-- 告警信息 -->
            <div class="alert-info">
              <div class="alert-header">
                <div class="alert-title">{{ alert.title }}</div>
                <div class="alert-badges">
                  <el-tag :type="getLevelType(alert.level)" size="small">
                    {{ alert.level }}
                  </el-tag>
                  <el-tag :type="getStateType(alert.alertState)" size="small">
                    {{ getStateText(alert.alertState) }}
                  </el-tag>
                </div>
              </div>

              <!-- 标签 -->
              <div class="alert-labels">
                <el-tag 
                  v-for="label in alert.labels" 
                  :key="label.key"
                  size="small" 
                  type="info" 
                  class="label-tag"
                >
                  {{ label.key }}={{ label.value }}
                </el-tag>
              </div>

              <!-- 时间信息 -->
              <div class="alert-time">
                {{ formatRelativeTime(alert.alertSince) }}
              </div>
            </div>

            <!-- 操作按钮 -->
            <div class="alert-actions">
              <el-button 
                v-if="canShowAnalysis(alert.alertState)"
                type="primary" 
                size="small" 
                @click="showAIAnalysis(alert)"
              >
                分析
              </el-button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- AI分析对话框 -->
    <el-dialog
      v-model="showAnalysisDialog"
      title="AI分析处理记录"
      width="80%"
      :close-on-click-modal="false"
      class="analysis-dialog"
    >
      <div v-if="selectedAlert" class="analysis-content">
        <!-- 告警基本信息 -->
        <div class="alert-basic-info">
          <div class="info-badges">
            <el-tag :type="getLevelType(selectedAlert.level)">
              {{ selectedAlert.level }}
            </el-tag>
            <el-tag :type="getStateType(selectedAlert.alertState)">
              {{ getStateText(selectedAlert.alertState) }}
            </el-tag>
          </div>
          <div class="alert-time-info">
            告警时间: {{ formatRelativeTime(selectedAlert.alertSince) }}
          </div>
        </div>

        <!-- 加载状态 -->
        <div v-if="detailLoading" class="loading-container">
          <el-icon class="is-loading"><Loading /></el-icon>
          <span>加载AI分析数据中...</span>
        </div>

        <!-- AI分析结果 -->
        <div v-else-if="alertDetail" class="ai-analysis-results">
          <div 
            v-for="(comment, index) in alertDetail.comments" 
            :key="index"
            class="analysis-result"
          >
            <div class="result-header">
              <el-icon class="brain-icon"><InfoFilled /></el-icon>
              <span class="result-title">AI分析结果</span>
              <span class="result-time">{{ formatRelativeTime(comment.createAt) }}</span>
            </div>
            <div class="result-content">
              <div class="markdown-content" v-html="renderMarkdown(comment.content)"></div>
            </div>
          </div>
        </div>

        <!-- 无数据状态 -->
        <div v-else class="no-data-container">
          <el-icon><Warning /></el-icon>
          <span>暂无AI分析数据</span>
        </div>

        <!-- 操作按钮 -->
        <div class="analysis-actions">
          <el-button 
            v-if="selectedAlert.alertState === 'InProcessing'"
            type="danger" 
            size="small"
            @click="executeRollback"
          >
            执行回滚
          </el-button>
          <el-button 
            v-if="selectedAlert.alertState === 'InProcessing'"
            type="default" 
            size="small"
            @click="markAsRestored"
          >
            标记恢复正常
          </el-button>
          <div 
            v-if="selectedAlert.alertState === 'Restored'"
            class="restored-status"
          >
            <el-icon class="check-icon"><Check /></el-icon>
            已恢复正常
          </div>
          <div 
            v-if="selectedAlert.alertState === 'AutoRestored'"
            class="restored-status"
          >
            <el-icon class="check-icon"><Check /></el-icon>
            系统自动恢复
          </div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { 
  ArrowLeft, 
  Warning, 
  Check, 
  InfoFilled,
  Loading
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { apiService } from '@/api'
import type { AlertsResponse, AlertIssue, AlertDetail } from '@/mock/services'
import { marked } from 'marked'

const router = useRouter()

// 状态管理
const filterState = ref<'all' | 'open' | 'closed'>('all')
const showAnalysisDialog = ref(false)
const selectedAlert = ref<AlertIssue | null>(null)
const alertDetail = ref<AlertDetail | null>(null)
const alerts = ref<AlertIssue[]>([])
const allAlerts = ref<AlertIssue[]>([]) // 存储所有告警数据用于计数
const loading = ref(false)
const detailLoading = ref(false)
const error = ref<string | null>(null)

// 计算属性
const filters = computed(() => [
  { 
    key: 'all', 
    label: 'All', 
    count: allAlerts.value.length 
  },
  { 
    key: 'open', 
    label: 'Open', 
    count: allAlerts.value.filter(alert => alert.state === 'Open').length 
  },
  { 
    key: 'closed', 
    label: 'Closed', 
    count: allAlerts.value.filter(alert => alert.state === 'Closed').length 
  }
])


// 方法
const setFilterState = (state: 'all' | 'open' | 'closed') => {
  filterState.value = state
  loadAlerts() // 触发重新加载
}

const formatRelativeTime = (timestamp: string) => {
  const now = new Date()
  const alertTime = new Date(timestamp)
  const diffMs = now.getTime() - alertTime.getTime()
  const diffMinutes = Math.floor(diffMs / (1000 * 60))
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60))
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))
  const diffMonths = Math.floor(diffDays / 30)

  if (diffMonths >= 1) {
    return alertTime.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit'
    })
  } else if (diffDays >= 1) {
    return `${diffDays}天前`
  } else if (diffHours >= 1) {
    return `${diffHours}小时前`
  } else if (diffMinutes >= 1) {
    return `${diffMinutes}分钟前`
  } else {
    return '刚刚'
  }
}

const getLevelType = (level: string) => {
  switch (level) {
    case 'P0':
    case 'P1':
      return 'danger'
    case 'P2':
      return 'warning'
    case 'Warning':
      return 'warning'
    default:
      return 'info'
  }
}

const getStateType = (alertState: string) => {
  switch (alertState) {
    case 'Pending':
      return 'danger'
    case 'InProcessing':
      return 'warning'
    case 'Restored':
    case 'AutoRestored':
      return 'success'
    default:
      return 'info'
  }
}

const getStateText = (alertState: string) => {
  switch (alertState) {
    case 'Pending':
      return '待处理'
    case 'InProcessing':
      return '处理中'
    case 'Restored':
      return '已恢复'
    case 'AutoRestored':
      return '系统自动恢复'
    default:
      return alertState
  }
}

const canShowAnalysis = (alertState: string) => {
  return ['Pending', 'InProcessing', 'Restored', 'AutoRestored'].includes(alertState)
}

// Markdown渲染方法
const renderMarkdown = (content: string) => {
  try {
    return marked.parse(content)
  } catch (error) {
    console.error('Markdown渲染失败:', error)
    return content // 如果渲染失败，返回原始内容
  }
}

const showAIAnalysis = async (alert: AlertIssue) => {
  try {
    selectedAlert.value = alert
    showAnalysisDialog.value = true
    detailLoading.value = true
    
    // 调用API获取告警详情（真实后端）
    const detailResp = await apiService.getAlertDetail(alert.id)
    console.log('告警详情响应 data:', detailResp.data)
    alertDetail.value = detailResp.data
    
    console.log('告警详情加载成功:', detailResp.data)
  } catch (err) {
    console.error('加载告警详情失败:', err)
    ElMessage.error('加载告警详情失败')
    showAnalysisDialog.value = false
  } finally {
    detailLoading.value = false
  }
}

const executeRollback = () => {
  ElMessage.success('执行回滚操作，跳转到系统变更记录页面')
  showAnalysisDialog.value = false
  router.push('/changelog')
}

const markAsRestored = () => {
  ElMessage.success('标记为恢复正常')
  showAnalysisDialog.value = false
}

// 原来的死数据已移动到mock/services.ts中，现在通过API调用获取

// 加载告警数据
const loadAlerts = async () => {
  try {
    loading.value = true
    error.value = null

    if (filterState.value === 'all') {
      // All 需要同时包含 Open 和 Closed，后端未传 state 时可能默认仅返回 Open
      const [openResp, closedResp] = await Promise.all([
        apiService.getAlerts(undefined, 100, 'Open'),
        apiService.getAlerts(undefined, 100, 'Closed')
      ])
      const merged = [...openResp.data.items, ...closedResp.data.items]
        .sort((a: any, b: any) => new Date(b.alertSince).getTime() - new Date(a.alertSince).getTime())

      alerts.value = merged.slice(0, 10)
      allAlerts.value = merged
      console.log('告警数据加载成功: All', { total: merged.length })
    } else {
      const state = filterState.value === 'open' ? 'Open' : 'Closed'

      // 并行请求当前筛选列表，以及用于右上角计数的全量 Open/Closed
      const [listResp, openResp, closedResp] = await Promise.all([
        apiService.getAlerts(undefined, 10, state),
        apiService.getAlerts(undefined, 100, 'Open'),
        apiService.getAlerts(undefined, 100, 'Closed')
      ])

      alerts.value = listResp.data.items

      const mergedAll = [...openResp.data.items, ...closedResp.data.items]
        .sort((a: any, b: any) => new Date(b.alertSince).getTime() - new Date(a.alertSince).getTime())
      allAlerts.value = mergedAll
      console.log('告警数据加载成功:', { filter: state, count: alerts.value.length, total: mergedAll.length })
    }
  } catch (err) {
    console.error('加载告警数据失败:', err)
    error.value = '加载告警数据失败'
    ElMessage.error('加载告警数据失败')
  } finally {
    loading.value = false
  }
}

// 生命周期
onMounted(() => {
  loadAlerts()
})
</script>

<style scoped>
.alerts-page {
  width: 100vw;
  min-height: 100vh;
  background: linear-gradient(to bottom, #f8fafc, #ffffff);
  padding: 0;
  margin: 0;
  box-sizing: border-box;
  overflow-x: hidden;
}

.alerts-page .content-wrapper {
  max-width: 1200px;
  margin: 0 auto;
  padding: 24px;
  width: 100%;
  box-sizing: border-box;
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

.back-button {
  display: flex;
  align-items: center;
  gap: 4px;
}

.divider {
  width: 1px;
  height: 20px;
  background-color: #e2e8f0;
}

.title {
  font-size: 20px;
  font-weight: 600;
  color: #1e293b;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  color: #1e293b;
  margin-bottom: 16px;
}

.filter-section {
  margin-bottom: 24px;
}

.loading-container {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 40px;
  color: #666;
  font-size: 14px;
}

.error-container {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 40px;
  color: #f56c6c;
  font-size: 14px;
}

.no-data-container {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 40px;
  color: #909399;
  font-size: 14px;
}

.markdown-content {
  word-wrap: break-word;
  font-size: 14px;
  line-height: 1.6;
  background: #f8f9fa;
  padding: 16px;
  border-radius: 6px;
  border: 1px solid #e9ecef;
  margin: 0;
}

/* Markdown内容样式 */
.markdown-content h1,
.markdown-content h2,
.markdown-content h3,
.markdown-content h4,
.markdown-content h5,
.markdown-content h6 {
  margin-top: 0;
  margin-bottom: 12px;
  font-weight: 600;
  color: #1e293b;
}

.markdown-content h1 { font-size: 20px; }
.markdown-content h2 { font-size: 18px; }
.markdown-content h3 { font-size: 16px; }

.markdown-content p {
  margin: 0 0 12px 0;
  color: #374151;
}

.markdown-content ul,
.markdown-content ol {
  margin: 0 0 12px 0;
  padding-left: 20px;
}

.markdown-content li {
  margin-bottom: 4px;
  color: #374151;
}

.markdown-content strong {
  font-weight: 600;
  color: #1e293b;
}

.markdown-content code {
  background: #e5e7eb;
  padding: 2px 6px;
  border-radius: 3px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 13px;
}

.markdown-content pre {
  background: #1f2937;
  color: #f9fafb;
  padding: 12px;
  border-radius: 6px;
  overflow-x: auto;
  margin: 12px 0;
}

.markdown-content pre code {
  background: none;
  padding: 0;
  color: inherit;
}

.result-time {
  color: #909399;
  font-size: 12px;
  margin-left: auto;
}

.filter-buttons {
  display: flex;
  gap: 16px;
}

.filter-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 6px;
  border: none;
  background: transparent;
  color: #64748b;
  font-size: 14px;
  cursor: pointer;
  transition: all 0.2s;
}

.filter-btn:hover {
  color: #1e293b;
}

.filter-btn.active {
  background-color: #f1f5f9;
  color: #1e293b;
  font-weight: 500;
}

.count-badge {
  background-color: #f1f5f9;
  color: #475569;
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 12px;
}

.filter-btn.active .count-badge {
  background-color: #e2e8f0;
  color: #334155;
}

.alerts-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.alert-card {
  background: white;
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  border: 1px solid #e2e8f0;
}

.alert-content {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 16px;
}

.status-icon {
  flex-shrink: 0;
  margin-top: 4px;
}

.open-icon {
  color: #ef4444;
  font-size: 16px;
}

.closed-icon {
  color: #22c55e;
  font-size: 16px;
}

.alert-info {
  flex: 1;
}

.alert-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.alert-title {
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
  line-height: 1.4;
}

.alert-badges {
  display: flex;
  gap: 8px;
}

.alert-labels {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-bottom: 8px;
}

.label-tag {
  font-size: 12px;
}

.alert-time {
  font-size: 12px;
  color: #64748b;
}

.alert-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.analysis-dialog {
  max-height: 80vh;
}

.analysis-content {
  max-height: 60vh;
  overflow-y: auto;
}

.alert-basic-info {
  background-color: #f8fafc;
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 16px;
}

.info-badges {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}

.alert-time-info {
  font-size: 14px;
  color: #64748b;
}

.ai-analysis-results {
  margin-bottom: 16px;
}

.analysis-result {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 16px;
}

.result-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.brain-icon {
  color: #3b82f6;
  font-size: 16px;
}

.result-title {
  font-weight: 500;
  color: #1e293b;
}

.result-time {
  font-size: 12px;
  color: #64748b;
  margin-left: auto;
}

.result-content {
  background-color: #f8fafc;
  border-radius: 6px;
  padding: 12px;
}

.markdown-content {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  font-size: 14px;
  line-height: 1.6;
  color: #374151;
  white-space: pre-wrap;
  margin: 0;
}

.analysis-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-top: 16px;
  border-top: 1px solid #e2e8f0;
}

.restored-status {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: #22c55e;
}

.check-icon {
  font-size: 16px;
}
</style>