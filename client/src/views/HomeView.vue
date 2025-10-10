<template>
  <div class="home-container">
    <!-- 顶部导航 -->
    <div class="header">
      <div class="title">Zero Ops</div>
      <div class="nav-buttons">
        <el-button type="warning" @click="goToAlerts" class="nav-btn">
          <el-icon><Warning /></el-icon>
          告警记录
        </el-button>
        <el-button type="primary" @click="goToChangelog" class="nav-btn">
          系统变更记录
        </el-button>
      </div>
    </div>

    <div class="subtitle">整体服务状态</div>

    <!-- 服务关系图 -->
    <el-card class="topology-card" shadow="hover">
      <template #header>
        <div class="card-header">
          <el-icon><Connection /></el-icon>
          <span>服务关系图</span>
        </div>
      </template>
      
      <div class="topology-container">
        <!-- 加载状态 -->
        <div v-if="loading" class="loading-container">
          <el-icon class="is-loading"><Loading /></el-icon>
          <span>加载服务数据中...</span>
        </div>
        
        <!-- 错误状态 -->
        <div v-else-if="error" class="error-container">
          <el-icon><Warning /></el-icon>
          <span>{{ error }}</span>
          <el-button size="small" @click="loadServicesData">重试</el-button>
        </div>
        
        <!-- SVG 连接线 -->
        <svg class="edges-svg">
          <defs>
            <template v-for="(edge, idx) in edges" :key="`arrow-${idx}`">
              <marker
                :id="`arrow-${idx}`"
                markerWidth="8"
                markerHeight="8"
                refX="8"
                refY="4"
                orient="auto"
              >
                <path d="M0,0 L0,8 L8,4 z" fill="#94a3b8" />
              </marker>
            </template>
          </defs>
          <template v-for="(edge, idx) in edges" :key="idx">
            <line
              :x1="getNodePosition(edge.source).x"
              :y1="getNodePosition(edge.source).y"
              :x2="getNodePosition(edge.target).x"
              :y2="getNodePosition(edge.target).y"
              stroke="#cbd5e1"
              stroke-width="2"
              :marker-end="`url(#arrow-${idx})`"
            />
          </template>
        </svg>

        <!-- 服务节点 -->
        <div
          v-for="node in nodes"
          :key="node.id"
          class="service-node"
          :style="{
            left: `${node.x}px`,
            top: `${node.y}px`
          }"
          @click="handleNodeClick(node)"
        >
          <div 
            class="node-circle"
            :style="{ backgroundColor: getNodeStatusColor(node) }"
          >
            <span class="node-name">{{ node.name }}</span>
            <!-- 灰度发布指示器 -->
            <div 
              v-if="hasRollingVersion(node)"
              class="rolling-indicator"
            >
              <svg viewBox="0 0 24 24">
                <circle
                  cx="12"
                  cy="12"
                  r="10"
                  fill="none"
                  :stroke="getNodeStatusStroke(node)"
                  stroke-width="2"
                />
                <path
                  d="M 12 2 A 10 10 0 0 1 12 22"
                  :fill="getNodeStatusFill(node)"
                  opacity="0.3"
                />
              </svg>
            </div>
          </div>
        </div>

        <!-- 图例说明 -->
        <div class="legend">
          <div class="legend-title">图例说明</div>
          <div class="legend-item">
            <span class="legend-dot" style="background-color: #10b981;"></span>
            <span>：服务正常</span>
          </div>
          <div class="legend-item">
            <span class="legend-dot" style="background-color: #f59e0b;"></span>
            <span>：有异常，AI正在观察和分析</span>
          </div>
          <div class="legend-item">
            <span class="legend-dot" style="background-color: #f43f5e;"></span>
            <span>：服务有异常</span>
          </div>
          <div class="legend-divider"></div>
          <div class="legend-item">
            <div class="rolling-example">
              <svg viewBox="0 0 24 24">
                <circle cx="12" cy="12" r="10" fill="none" stroke="#10b981" stroke-width="2" />
                <path d="M 12 2 A 10 10 0 0 1 12 22" fill="#10b981" opacity="0.3" />
              </svg>
            </div>
            <span>：服务正在灰度发布中</span>
          </div>
        </div>
      </div>
    </el-card>

    <!-- 服务详情弹窗 -->
    <el-dialog
      v-model="dialogVisible"
      title="服务概览"
      width="90%"
      :fullscreen="false"
      @close="handleCloseDialog"
    >
      <div v-if="selectedNode" class="service-detail">
        <div class="detail-header">
          <span>{{ selectedNode.name }} 服务概览</span>
          <el-tag :type="getStatusType(getNodeStatus(selectedNode))">
            {{ getNodeStatusText(getNodeStatus(selectedNode)) }}
          </el-tag>
        </div>
        <p class="detail-desc">点击饼图扇区进入指标看板（四大黄金指标）。</p>
        
        <div class="detail-content">
          <!-- 饼图 -->
          <div class="pie-chart-container">
            <div ref="pieChartRef" class="pie-chart"></div>
          </div>
          
          <!-- 版本信息表格 -->
          <div class="version-info">
            <div class="version-title">各版本：延迟、流量、错误、饱和度</div>
            <el-table :data="getVersionTableData(selectedNode.versions)" size="small" class="version-table">
              <el-table-column prop="version" label="版本" width="80" />
              <el-table-column prop="latency" label="延迟" width="80" />
              <el-table-column prop="traffic" label="流量" width="80" />
              <el-table-column prop="errors" label="错误" width="80" />
              <el-table-column prop="saturation" label="饱和度" width="80" />
              <el-table-column prop="status" label="状态" width="80">
                <template #default="{ row }">
                  <el-tag 
                    :type="getStatusType(row.status)"
                    size="small"
                  >
                    {{ row.status }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column width="120">
                <template #default="{ row }">
                  <div class="table-actions">
                    <span 
                      :class="['action-link', row.isPaused ? 'continue' : 'pause']" 
                      @click="togglePauseResumeForVersion(row)"
                    >
                      {{ row.isPaused ? '继续' : '暂停' }}
                    </span>
                    <span 
                      class="action-link rollback" 
                      @click="rollbackVersion(row)"
                    >
                      回滚
                    </span>
                  </div>
                </template>
              </el-table-column>
              <template #empty>
                <div class="no-data">暂无数据</div>
              </template>
            </el-table>
            
            <!-- 发布管理 -->
            <div class="release-controls">
              <el-select v-model="selectedVersion" placeholder="选择目标版本" style="width: 250px">
                <el-option
                  v-for="option in availableVersionOptions"
                  :key="option.value"
                  :label="option.label"
                  :value="option.value"
                />
              </el-select>
              
              <el-date-picker
                v-model="scheduledStart"
                type="datetime"
                placeholder="发布起始时间"
                format="YYYY-MM-DD HH:mm:ss"
                value-format="YYYY-MM-DD HH:mm:ss"
                style="width: 200px"
              />
              
              <el-button type="primary" @click="createRelease">新建发布</el-button>
              
              <el-popover placement="bottom" width="300" trigger="click">
                <template #reference>
                  <el-button>
                    <el-icon><Clock /></el-icon>
                    计划
                  </el-button>
                </template>
                <div class="release-plans">
                  <div class="plans-header">{{ selectedNode.name }} 发布计划列表</div>
                  <div v-if="deploymentPlansForDisplay.length > 0" class="plans-list">
                    <div
                      v-for="plan in deploymentPlansForDisplay"
                      :key="plan.id"
                      class="plan-item"
                    >
                      <div class="plan-content">
                        <div class="plan-info">
                          <div class="plan-header">
                            <span class="plan-version">{{ plan.version }}</span>
                          </div>
                          <div class="plan-details">
                            <div>{{ plan.time }}</div>
                            <div>状态: 
                              <el-tag 
                                :type="plan.originalStatus === 'Finished' ? 'success' : 
                                       plan.originalStatus === 'Schedule' ? 'info' : 'warning'"
                                size="small"
                              >
                                {{ plan.status }}
                              </el-tag>
                            </div>
                          </div>
                        </div>
         <!-- 待发布和发布中的计划：编辑、取消 -->
         <div class="plan-actions" v-if="plan.originalStatus === 'Schedule' || plan.originalStatus === 'InDeployment'">
           <div class="action-column">
             <span class="action-link edit" @click="editRelease(plan)">编辑</span>
             <span class="action-link cancel" @click="confirmCancel(plan)">取消</span>
           </div>
         </div>
         
         <!-- 已完成的计划：只显示状态，无操作按钮 -->
         <div class="plan-actions" v-else-if="plan.originalStatus === 'Finished'">
           <div class="action-column">
             <span class="completed-text">已完成</span>
           </div>
         </div>
                      </div>
                    </div>
                  </div>
                  <div v-else class="no-plans">暂无发布计划</div>
                </div>
              </el-popover>
            </div>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- 指标展示弹窗 -->
    <el-dialog
      v-model="metricsDialogVisible"
      title="服务指标详情"
      width="70%"
      :close-on-click-modal="false"
      top="2vh"
      @close="handleCloseMetricsDialog"
    >
      <div class="metrics-dialog-content">
        <div class="metrics-header">
          <h3>{{ selectedNode?.name }} 服务指标详情</h3>
          <p class="metrics-subtitle">四大黄金指标监控</p>
        </div>
        
        <div class="metrics-grid">
          <!-- 延迟指标 -->
          <div class="metric-panel">
            <div class="metric-header">
              <h4>Latency (p95)</h4>
              <p class="metric-note">指标说明：请求延迟 p95</p>
            </div>
            <div class="metric-value">
              <span class="value">{{ getCurrentMetricValue('latency') }}</span>
              <span class="unit">ms</span>
            </div>
            <div class="metric-chart" ref="latencyChartRef"></div>
          </div>

          <!-- 流量指标 -->
          <div class="metric-panel">
            <div class="metric-header">
              <h4>Traffic (qps)</h4>
              <p class="metric-note">指标说明：每秒请求数</p>
            </div>
            <div class="metric-value">
              <span class="value">{{ getCurrentMetricValue('traffic') }}</span>
              <span class="unit">qps</span>
            </div>
            <div class="metric-chart" ref="trafficChartRef"></div>
          </div>

          <!-- 错误率指标 -->
          <div class="metric-panel">
            <div class="metric-header">
              <h4>Errors (%)</h4>
              <p class="metric-note">指标说明：错误率</p>
            </div>
            <div class="metric-value">
              <span class="value">{{ getCurrentMetricValue('errors') }}</span>
              <span class="unit">%</span>
            </div>
            <div class="metric-chart" ref="errorsChartRef"></div>
          </div>

          <!-- 饱和度指标 -->
          <div class="metric-panel">
            <div class="metric-header">
              <h4>Saturation (%)</h4>
              <p class="metric-note">指标说明：资源饱和度</p>
            </div>
            <div class="metric-value">
              <span class="value">{{ getCurrentMetricValue('saturation') }}</span>
              <span class="unit">%</span>
            </div>
            <div class="metric-chart" ref="saturationChartRef"></div>
          </div>
        </div>
      </div>
      
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="handleCloseMetricsDialog">返回服务详情页</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 编辑部署计划对话框 -->
    <el-dialog
      v-model="showEditDialog"
      title="编辑部署计划"
      width="500px"
      :before-close="cancelEdit"
    >
      <div class="edit-deployment-form">
        <div class="form-item">
          <label class="form-label">服务名称</label>
          <div class="form-value">{{ editingDeployment?.service || selectedNode?.name || '未知服务' }}</div>
        </div>
        
        <div class="form-item">
          <label class="form-label">版本号</label>
          <div class="form-value">{{ editingDeployment?.version }}</div>
        </div>
        
        <div class="form-item">
          <label class="form-label">计划发布时间 <span class="required">*</span></label>
          <el-date-picker
            v-model="editForm.scheduleTime"
            type="datetime"
            placeholder="选择发布时间"
            format="YYYY-MM-DD HH:mm"
            value-format="YYYY-MM-DDTHH:mm"
            style="width: 100%"
          />
        </div>
      </div>
      
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="cancelEdit">取消</el-button>
          <el-button type="primary" @click="saveEditDeployment">保存</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Loading, Warning } from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import { apiService } from '@/api'
import { mockApi } from '@/mock/api'
import type { ServicesResponse, ServiceDetail, ServiceActiveVersionsResponse, ServiceMetricsResponse, AvailableVersionsResponse, DeploymentPlansResponse, MetricsResponse } from '@/mock/services'
import { updateServiceAlertStatus, getServiceAlertStatus, type ServiceAlertStatus, getServiceDeploymentStatus, type DeploymentStatus, getServiceVersionAlertStatus } from '@/mock/services'

const router = useRouter()

// 响应式数据
const dialogVisible = ref(false)
const selectedNode = ref<any>(null)
const selectedVersion = ref('v1.0.7')
const scheduledStart = ref('')
const pieChartRef = ref<HTMLElement>()

// 服务数据
const servicesData = ref<ServicesResponse | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

// 存储当前服务的指标数据
const currentServiceMetrics = ref<ServiceMetricsResponse | null>(null)

// 存储当前服务的可发布版本列表
const currentServiceAvailableVersions = ref<AvailableVersionsResponse | null>(null)

// 存储当前服务的发布计划列表
const currentServiceDeploymentPlans = ref<DeploymentPlansResponse | null>(null)

// 指标展示相关状态
const metricsDialogVisible = ref(false)
const selectedSlice = ref<{ nodeId: string; label: string } | null>(null)

// 指标图表引用
const latencyChartRef = ref<HTMLElement>()
const trafficChartRef = ref<HTMLElement>()

// 编辑部署对话框状态
const showEditDialog = ref(false)
const editingDeployment = ref<any>(null)
const editForm = ref({
  scheduleTime: ''
})
const errorsChartRef = ref<HTMLElement>()
const saturationChartRef = ref<HTMLElement>()

// 存储指标数据
const metricsData = ref<{
  latency?: MetricsResponse
  traffic?: MetricsResponse
  errors?: MetricsResponse
  saturation?: MetricsResponse
}>({})

// 自动布局算法
const calculateAutoLayout = (services: any[]) => {
  // 布局配置
  const layoutConfig = {
    levelHeight: 150,      // 层级间距
    nodeSpacing: 200,      // 同层节点间距
    startX: 400,           // 起始X坐标
    startY: 100,           // 起始Y坐标
    maxNodesPerLevel: 6    // 每层最大节点数
  }
  
  // 1. 构建依赖图
  const dependencyGraph = new Map<string, string[]>()
  const reverseGraph = new Map<string, string[]>()
  
  services.forEach(service => {
    dependencyGraph.set(service.name, service.deps || [])
    reverseGraph.set(service.name, [])
  })
  
  // 构建反向图
  services.forEach(service => {
    service.deps?.forEach((dep: string) => {
      if (reverseGraph.has(dep)) {
        reverseGraph.get(dep)!.push(service.name)
      }
    })
  })
  
  // 2. 拓扑排序确定层级
  const levels: string[][] = []
  const visited = new Set<string>()
  const inDegree = new Map<string, number>()
  
  // 计算入度
  services.forEach(service => {
    inDegree.set(service.name, service.deps?.length || 0)
  })
  
  // 找到所有入度为0的节点（根节点）
  let currentLevel: string[] = []
  inDegree.forEach((degree, serviceName) => {
    if (degree === 0) {
      currentLevel.push(serviceName)
    }
  })
  
  // 分层处理
  while (currentLevel.length > 0) {
    levels.push([...currentLevel])
    const nextLevel: string[] = []
    
    currentLevel.forEach(serviceName => {
      visited.add(serviceName)
      // 找到依赖当前服务的所有服务
      const dependents = reverseGraph.get(serviceName) || []
      dependents.forEach(dependent => {
        if (!visited.has(dependent)) {
          const currentDegree = inDegree.get(dependent) || 0
          inDegree.set(dependent, currentDegree - 1)
          if (inDegree.get(dependent) === 0) {
            nextLevel.push(dependent)
          }
        }
      })
    })
    
    currentLevel = nextLevel
  }
  
  
  // 3. 计算位置
  const positions = new Map<string, {x: number, y: number}>()
  
  levels.forEach((level, levelIndex) => {
    const levelY = layoutConfig.startY + levelIndex * layoutConfig.levelHeight
    const levelWidth = (level.length - 1) * layoutConfig.nodeSpacing
    const startX = layoutConfig.startX - levelWidth / 2
    
    level.forEach((serviceName, nodeIndex) => {
      const x = startX + nodeIndex * layoutConfig.nodeSpacing
      positions.set(serviceName, { x, y: levelY })
    })
  })
  
  return positions
}

// 数据转换函数
const transformServiceData = (data: ServicesResponse) => {
  const nodes: any[] = []
  const edges: any[] = []
  
  // 使用自动布局算法计算位置
  const positions = calculateAutoLayout(data.items)
  
  // 转换服务节点
  data.items.forEach((service) => {
    const position = positions.get(service.name) || { x: 400, y: 100 }
    
    const node = {
      id: service.name,
      name: service.name,
      x: position.x,
      y: position.y,
      health: service.health,
      deployState: service.deployState,
      dependencies: service.deps,
      // 根据发布状态生成版本信息
      versions: generateVersionsFromDeployState(service)
    }
    nodes.push(node)
    
    // 生成依赖关系边
    service.deps.forEach(dep => {
      edges.push({
        source: service.name,
        target: dep
      })
    })
  })
  
  return { nodes, edges }
}

// 根据发布状态生成版本信息
const generateVersionsFromDeployState = (service: any) => {
  if (service.deployState === 'deploying') {
    // 发布中：生成多个版本，其中一个在发布
    return [
      { label: "v1.0.0", value: 70, eta: "~ 2h 30m", anomalous: false, observing: false },
      { label: "v1.0.1", value: 30, eta: "~ 1h 10m", anomalous: false, observing: true, rolling: true, elapsedMin: 30, remainingMin: 60 }
    ]
  } else {
    // 发布完成：只有一个稳定版本
    const isError = service.health === 'Error'
    return [
      { 
        label: "v1.0.0", 
        value: 100, 
        eta: "~ 2h 30m", 
        anomalous: isError, 
        observing: false 
      }
    ]
  }
}

// API调用函数
const loadServicesData = async () => {
  loading.value = true
  error.value = null
  
  try {
    // 加载服务数据 - 调用真实后端API
    const response = await apiService.getServices()
    const servicesResponse = response.data
    
    servicesData.value = servicesResponse
    
    // 转换数据
    try {
      const { nodes: transformedNodes, edges: transformedEdges } = transformServiceData(servicesResponse)
      nodes.value = transformedNodes
      edges.value = transformedEdges
    } catch (transformError) {
      console.error('数据转换失败:', transformError)
      throw transformError
    }
    
    console.log('服务数据加载成功，返回值数据:', servicesResponse)
  } catch (err) {
    error.value = '加载服务数据失败'
    console.error('加载服务数据失败:', err)
    ElMessage.error('加载服务数据失败')
  } finally {
    loading.value = false
  }
}

// 数据转换函数：将后端返回的活跃版本数据转换为前端需要的格式
const transformActiveVersionsToFrontend = (activeVersionsResponse: ServiceActiveVersionsResponse, serviceName: string) => {
  // 添加空值检查
  if (!activeVersionsResponse || !activeVersionsResponse.items) {
    console.warn('活跃版本数据为空:', activeVersionsResponse)
    return []
  }
  
  const totalInstances = activeVersionsResponse.items.reduce((sum, item) => sum + item.instances, 0)
  
  return activeVersionsResponse.items.map(item => {
    // 计算百分比
    const percentage = totalInstances > 0 ? Math.round((item.instances / totalInstances) * 100) : 0
    
    // 计算ETA（预估剩余时间）
    const startTime = new Date(item.startTime)
    const estimatedCompletion = new Date(item.estimatedCompletionTime)
    const now = new Date()
    const remainingMs = estimatedCompletion.getTime() - now.getTime()
    const remainingMinutes = Math.max(0, Math.round(remainingMs / (1000 * 60)))
    const eta = remainingMinutes > 60 ? `~ ${Math.round(remainingMinutes / 60)}h ${remainingMinutes % 60}m` : `~ ${remainingMinutes}m`
    
    // 计算已用时间
    const elapsedMs = now.getTime() - startTime.getTime()
    const elapsedMinutes = Math.max(0, Math.round(elapsedMs / (1000 * 60)))
    
    // 判断是否在发布中（基于时间）
    const isRolling = remainingMinutes > 0 && elapsedMinutes > 0
    
    // 状态映射
    const isAnomalous = item.health === 'Error'
    const isObserving = item.health === 'Warning'
    
    // 读取版本级别告警状态以驱动颜色
    const versionStatus = getServiceVersionAlertStatus(serviceName, item.version)
    const versionAnomalous = versionStatus === 'pending'
    const versionObserving = versionStatus === 'processing'

    return {
      label: item.version,
      value: percentage,
      eta: eta,
      anomalous: versionAnomalous || isAnomalous,
      observing: versionObserving || (!versionAnomalous && isObserving),
      rolling: isRolling,
      elapsedMin: elapsedMinutes,
      remainingMin: remainingMinutes,
      deployID: item.deployID,
      startTime: item.startTime,
      estimatedCompletionTime: item.estimatedCompletionTime,
      instances: item.instances,
      health: item.health
    }
  })
}

// 获取服务详情 - 使用新的API接口
const loadServiceDetail = async (serviceName: string) => {
  try {
    // 调用活跃版本API - 调用真实后端API
    const response = await apiService.getServiceActiveVersions(serviceName)
    const activeVersionsResponse = response.data
    
    // 转换数据格式
    const transformedVersions = transformActiveVersionsToFrontend(activeVersionsResponse, serviceName)
    
    // 获取服务的基础信息（包括原始health状态）
    const serviceInfo = nodes.value.find(node => node.name === serviceName)
    
    // 根据告警状态确定最终的health状态
    const alertStatus = getServiceAlertStatus(serviceName)
    let finalHealth = serviceInfo?.health || 'Normal'
    
    // 如果服务有告警状态，覆盖原始health状态
    if (alertStatus === 'pending') {
      finalHealth = 'Error' // 红色
    } else if (alertStatus === 'processing') {
      finalHealth = 'Warning' // 黄色
    }
    
    return {
      name: serviceName,
      versions: transformedVersions,
      health: finalHealth,
      deployState: serviceInfo?.deployState || 'completed',
      deps: serviceInfo?.deps || []
    }
  } catch (err) {
    console.error('获取服务活跃版本失败:', err)
    ElMessage.error('获取服务详情失败')
    return null
  }
}

// 获取服务指标数据 - 调用真实后端API
const loadServiceMetrics = async (serviceName: string) => {
  try {
    // 调用真实后端API
    const response = await apiService.getServiceMetrics(serviceName)
    return response.data
  } catch (err) {
    console.warn(`获取服务 ${serviceName} 指标数据失败:`, err)
    // 不显示错误消息，因为某些服务可能没有指标数据
    return null
  }
}

// 获取服务可发布版本列表 - 调用真实后端API
const loadServiceAvailableVersions = async (serviceName: string) => {
  try {
    // 调用真实后端API
    const response = await apiService.getServiceAvailableVersions(serviceName)
    return response.data
  } catch (err) {
    console.error('获取服务可发布版本失败:', err)
    ElMessage.error('获取服务可发布版本失败')
    return null
  }
}

// 获取服务发布计划列表 - 调用真实后端API
const loadServiceDeploymentPlans = async (serviceName: string) => {
  try {
    // 调用真实后端API
    const response = await apiService.getServiceDeploymentPlans(serviceName)
    return response.data
  } catch (err) {
    console.error('获取服务发布计划失败:', err)
    ElMessage.error('获取服务发布计划失败')
    return null
  }
}

// 获取服务指标数据 - 调用真实后端API
const loadServiceMetricsData = async (serviceName: string, version: string) => {
  try {
    // 并行获取四大黄金指标数据 - 调用真实后端API
    const [latencyResponse, trafficResponse, errorsResponse, saturationResponse] = await Promise.all([
      apiService.getServiceMetricsData(serviceName, 'latency', version),
      apiService.getServiceMetricsData(serviceName, 'traffic', version),
      apiService.getServiceMetricsData(serviceName, 'errors', version),
      apiService.getServiceMetricsData(serviceName, 'saturation', version)
    ])
    
    const latencyData = latencyResponse.data
    const trafficData = trafficResponse.data
    const errorsData = errorsResponse.data
    const saturationData = saturationResponse.data
    
    return {
      latency: latencyData,
      traffic: trafficData,
      errors: errorsData,
      saturation: saturationData
    }
  } catch (err) {
    console.error('获取服务指标数据失败:', err)
    ElMessage.error('获取服务指标数据失败')
    return null
  }
}

// 生命周期
onMounted(() => {
  loadServicesData()
})

// 服务拓扑数据（通过API获取）
const nodes = ref<any[]>([])
const edges = ref<any[]>([])


// 计算属性：将可发布版本数据转换为下拉框格式
const availableVersionOptions = computed(() => {
  if (!currentServiceAvailableVersions.value || !currentServiceAvailableVersions.value.items) {
    return []
  }
  
  return currentServiceAvailableVersions.value.items.map(item => ({
    label: `${item.version} (${new Date(item.createTime).toLocaleDateString('zh-CN')})`,
    value: item.version
  }))
})

// 计算属性：将发布计划数据转换为前端显示格式
const deploymentPlansForDisplay = computed(() => {
  if (!currentServiceDeploymentPlans.value || !currentServiceDeploymentPlans.value.items) {
    return []
  }
  
  return currentServiceDeploymentPlans.value.items.map(plan => {
    // 状态映射
    const statusMap = {
      'Schedule': '待发布',
      'InDeployment': '部署中',
      'Finished': '已完成'
    }
    
    // 时间格式化
    const formatTime = (timeStr?: string) => {
      if (!timeStr) return '已开始'
      return new Date(timeStr).toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit'
      })
    }
    
    // 根据状态生成时间显示
    const generateTimeDisplay = () => {
      switch (plan.status) {
        case 'InDeployment':
          // 部署中：显示开始时间或"已开始"
          if (plan.scheduleTime) {
            return `开始时间: ${formatTime(plan.scheduleTime)}`
          } else {
            return '已开始'
          }
        case 'Finished':
          // 已完成：显示开始时间 → 结束时间
          if (plan.scheduleTime && plan.finishTime) {
            return `开始时间: ${formatTime(plan.scheduleTime)} → 结束时间: ${formatTime(plan.finishTime)}`
          } else if (plan.finishTime) {
            return `结束时间: ${formatTime(plan.finishTime)}`
          } else {
            return '已完成'
          }
        default:
          return '未知状态'
      }
    }
    
    return {
      id: plan.id,
      service: plan.service, // 保留服务名称字段
      version: plan.version,
      status: statusMap[plan.status] || plan.status,
      time: generateTimeDisplay(),
      originalStatus: plan.status,
      isPaused: plan.isPaused || false, // 添加暂停状态，默认为false
      scheduleTime: plan.scheduleTime // 添加scheduleTime字段用于判断是否已开始
    }
  })
})

// 方法
const goToChangelog = () => {
  router.push('/changelog')
}

const goToAlerts = () => {
  router.push('/alerts')
}

const getNodeStatus = (node: any) => {
  // 首先检查告警状态
  const alertStatus = getServiceAlertStatus(node.name)
  
  // 如果服务有告警状态，优先使用告警状态
  if (alertStatus !== 'normal') {
    return alertStatus
  }
  
  // 否则使用后端返回的health状态
  const healthMap: Record<string, string> = {
    'Normal': 'healthy',
    'Warning': 'canary',
    'Error': 'abnormal'
  }
  return healthMap[node.health] || 'healthy'
}

const getNodeStatusColor = (node: any) => {
  const status = getNodeStatus(node)
  const statusMap: Record<string, string> = {
    healthy: "#10b981",    // 绿色 - 正常
    abnormal: "#f43f5e",   // 红色 - 异常
    canary: "#f59e0b",     // 黄色 - 观察中
    pending: "#ef4444",    // 红色 - 待处理
    processing: "#eab308"  // 黄色 - 处理中
  }
  return statusMap[status] || "#6b7280"
}

const getNodeStatusStroke = (node: any) => {
  const status = getNodeStatus(node)
  const statusMap: Record<string, string> = {
    healthy: "#10b981",
    abnormal: "#f43f5e",
    canary: "#f59e0b",
    pending: "#ef4444",
    processing: "#eab308"
  }
  return statusMap[status] || "#6b7280"
}

const getNodeStatusFill = (node: any) => {
  const status = getNodeStatus(node)
  const statusMap: Record<string, string> = {
    healthy: "#10b981",
    abnormal: "#f43f5e", 
    canary: "#f59e0b",
    pending: "#ef4444",
    processing: "#eab308"
  }
  return statusMap[status] || "#6b7280"
}

const hasRollingVersion = (node: any) => {
  // 检查发布状态或deployState判断是否显示灰度发布指示器
  const deploymentStatus = getServiceDeploymentStatus(node.name)
  return deploymentStatus === 'deploying' || node.deployState === 'deploying'
}

const getNodePosition = (nodeId: string) => {
  const node = nodes.value.find(n => n.id === nodeId)
  return node ? { x: node.x, y: node.y } : { x: 0, y: 0 }
}

const handleNodeClick = async (node: any) => {
  selectedNode.value = { ...node, status: getNodeStatus(node) }
  dialogVisible.value = true
  
  // 并行加载服务详情、指标数据、可发布版本和发布计划
  const [serviceDetail, metricsData, availableVersionsData, deploymentPlansData] = await Promise.allSettled([
    loadServiceDetail(node.name),
    loadServiceMetrics(node.name),
    loadServiceAvailableVersions(node.name),
    loadServiceDeploymentPlans(node.name)
  ])
  
  // 提取成功的结果
  const serviceDetailResult = serviceDetail.status === 'fulfilled' ? serviceDetail.value : null
  const metricsDataResult = metricsData.status === 'fulfilled' ? metricsData.value : null
  const availableVersionsDataResult = availableVersionsData.status === 'fulfilled' ? availableVersionsData.value : null
  const deploymentPlansDataResult = deploymentPlansData.status === 'fulfilled' ? deploymentPlansData.value : null
  
  if (serviceDetailResult) {
    // 更新节点的版本信息
    selectedNode.value.versions = serviceDetailResult.versions
  }
  
  if (metricsDataResult) {
    // 存储指标数据
    currentServiceMetrics.value = metricsDataResult
  }
  
  if (availableVersionsDataResult) {
    // 存储可发布版本数据
    currentServiceAvailableVersions.value = availableVersionsDataResult
    // 重置选中的版本为第一个可用版本
    if (availableVersionsDataResult.items && availableVersionsDataResult.items.length > 0) {
      selectedVersion.value = availableVersionsDataResult.items[0].version
    }
  }
  
  if (deploymentPlansDataResult) {
    // 存储发布计划数据
    currentServiceDeploymentPlans.value = deploymentPlansDataResult
  }
  
  nextTick(() => {
    initPieChart()
  })
}

const getStatusType = (status: string) => {
  switch (status) {
    case 'healthy': return 'success'
    case 'canary': return 'warning'
    case 'abnormal': return 'danger'
    default: return 'info'
  }
}

const getNodeStatusText = (status: string) => {
  switch (status) {
    case 'healthy': return '服务正常'
    case 'canary': return '有异常，AI正在观察和分析'
    case 'abnormal': return '服务有异常'
    case 'pending': return '告警待处理'
    case 'processing': return '告警处理中'
    default: return '未知状态'
  }
}

// 数据转换函数：将后端返回的指标数据转换为前端表格需要的格式
const transformMetricsToTableData = (versions: any[], metricsResponse: ServiceMetricsResponse | null) => {
  if (!metricsResponse) {
    // 如果没有指标数据，返回空数组
    return []
  }

  // 使用真实的指标数据
  return versions.map(v => {
    // 找到对应版本的指标数据
    const versionMetrics = metricsResponse.items.find(item => item.version === v.label)
    
    if (versionMetrics) {
      // 从指标数据中提取对应的值
      const latency = versionMetrics.metrics.find(m => m.name === 'latency')?.value || 0
      const traffic = versionMetrics.metrics.find(m => m.name === 'traffic')?.value || 0
      const errorRatio = versionMetrics.metrics.find(m => m.name === 'errorRatio')?.value || 0
      const saturation = versionMetrics.metrics.find(m => m.name === 'saturation')?.value || 0
      
      return {
        version: v.label,
        latency: latency,
        traffic: traffic,
        errors: errorRatio.toFixed(1),
        saturation: saturation,
        status: v.anomalous ? '异常' : (v.observing ? '观察中' : '正常')
      }
    } else {
      // 如果找不到对应版本的指标，使用summary数据
      const latency = metricsResponse.summary.metrics.find(m => m.name === 'latency')?.value || 0
      const traffic = metricsResponse.summary.metrics.find(m => m.name === 'traffic')?.value || 0
      const errorRatio = metricsResponse.summary.metrics.find(m => m.name === 'errorRatio')?.value || 0
      const saturation = metricsResponse.summary.metrics.find(m => m.name === 'saturation')?.value || 0
      
      return {
        version: v.label,
        latency: latency,
        traffic: traffic,
        errors: errorRatio.toFixed(1),
        saturation: saturation,
        status: v.anomalous ? '异常' : (v.observing ? '观察中' : '正常')
      }
    }
  })
}

const getVersionTableData = (versions: any[]) => {
  const tableData = transformMetricsToTableData(versions, currentServiceMetrics.value)
  
  // 为每个版本添加部署状态信息（用于操作按钮）
  return tableData.map(version => {
    // 检查是否有正在进行的部署
    const activeDeployment = currentServiceDeploymentPlans.value?.items?.find((plan: any) => 
      plan.version === version.version && plan.status === 'InDeployment'
    )
    
    return {
      ...version,
      isPaused: activeDeployment?.isPaused || false,
      deployId: activeDeployment?.id || version.version // 如果没有deployId，使用version作为标识
    }
  })
}

const handleCloseDialog = () => {
  selectedNode.value = null
  dialogVisible.value = false
  // 清理当前服务的数据
  currentServiceMetrics.value = null
  currentServiceAvailableVersions.value = null
  currentServiceDeploymentPlans.value = null
  // 释放饼图实例
  if (pieChart) {
    pieChart.dispose()
    pieChart = null
  }
}

// 处理指标弹窗关闭
const handleCloseMetricsDialog = () => {
  metricsDialogVisible.value = false
  selectedSlice.value = null
  // 清理指标图表
  disposeMetricsCharts()
  // 清理指标数据
  metricsData.value = {}
}

// 获取当前指标值
const getCurrentMetricValue = (metricName: keyof typeof metricsData.value) => {
  const metricData = metricsData.value[metricName]
  if (!metricData || !metricData.data.result[0]?.values.length) {
    return '--'
  }
  
  // 获取最后一个数据点的值
  const lastValue = metricData.data.result[0].values[metricData.data.result[0].values.length - 1]
  return parseFloat(lastValue[1]).toFixed(1)
}

const createRelease = async () => {
  // 表单验证
  if (!selectedNode.value?.name) {
    ElMessage.error('请先选择服务')
    return
  }
  
  if (!selectedVersion.value) {
    ElMessage.error('请选择目标版本')
    return
  }
  
  try {
    // 调用后端API创建部署计划
    const response = await apiService.createDeployment({
      service: selectedNode.value.name,
      version: selectedVersion.value,
      scheduleTime: scheduledStart.value || undefined
    })
    
    console.log('部署计划创建成功:', response.data)
    
    // 重新加载服务详情数据（饼状图会自动更新）
    const serviceDetailResult = await loadServiceDetail(selectedNode.value.name)
    if (serviceDetailResult) {
      selectedNode.value = { ...serviceDetailResult, status: getNodeStatus(serviceDetailResult) }
    }
    
    // 重新加载可发布版本数据（下拉框会自动更新）
    const availableVersionsResult = await loadServiceAvailableVersions(selectedNode.value.name)
    if (availableVersionsResult) {
      currentServiceAvailableVersions.value = availableVersionsResult
    }
    
    // 重新加载服务指标数据
    const metricsDataResult = await loadServiceMetrics(selectedNode.value.name)
    if (metricsDataResult) {
      currentServiceMetrics.value = metricsDataResult
    }
    
    // 重新加载发布计划列表
    const deploymentPlansDataResult = await loadServiceDeploymentPlans(selectedNode.value.name)
    if (deploymentPlansDataResult) {
      currentServiceDeploymentPlans.value = deploymentPlansDataResult
    }
    
    // 重新加载服务数据以更新拓扑图
    await loadServicesData()
    
    ElMessage.success('发布任务创建成功')
    
    // 重置表单
    selectedVersion.value = ''
    scheduledStart.value = ''
    
  } catch (error) {
    console.error('创建发布任务失败:', error)
    ElMessage.error('创建发布任务失败')
  }
}

const editRelease = (release: any) => {
  editingDeployment.value = release
  // 将计划时间转换为本地时间格式（YYYY-MM-DDTHH:mm）
  if (release.scheduleTime) {
    const date = new Date(release.scheduleTime)
    const year = date.getFullYear()
    const month = String(date.getMonth() + 1).padStart(2, '0')
    const day = String(date.getDate()).padStart(2, '0')
    const hours = String(date.getHours()).padStart(2, '0')
    const minutes = String(date.getMinutes()).padStart(2, '0')
    editForm.value.scheduleTime = `${year}-${month}-${day}T${hours}:${minutes}`
  } else {
    editForm.value.scheduleTime = ''
  }
  showEditDialog.value = true
}

// 保存编辑的部署计划
const saveEditDeployment = async () => {
  if (!editingDeployment.value) return
  
  try {
    // 将本地时间转换为ISO格式
    const scheduleTime = editForm.value.scheduleTime ? new Date(editForm.value.scheduleTime).toISOString() : undefined
    
    const result = await apiService.updateDeployment(editingDeployment.value.id, {
      scheduleTime: scheduleTime
    })
    
    if (result.status === 200) {
      ElMessage.success('部署计划已更新')
      showEditDialog.value = false
      // 刷新发布计划列表
      await loadServiceDeploymentPlans(selectedNode.value?.name || '')
    } else {
      ElMessage.error('更新部署计划失败')
    }
  } catch (error) {
    console.error('更新部署计划失败:', error)
    ElMessage.error('更新部署计划失败')
  }
}

// 取消编辑
const cancelEdit = () => {
  showEditDialog.value = false
  editingDeployment.value = null
  editForm.value.scheduleTime = ''
}

const confirmCancel = async (plan: any) => {
  try {
    // 调用后端API取消部署计划
    const response = await apiService.cancelDeployment(plan.id)
    
    console.log('取消部署计划成功:', response.data)
    ElMessage.success('发布计划已取消')
    
    // 刷新发布计划列表
    if (selectedNode.value) {
      const deploymentPlansDataResult = await loadServiceDeploymentPlans(selectedNode.value.name)
      if (deploymentPlansDataResult) {
        currentServiceDeploymentPlans.value = deploymentPlansDataResult
      }
    }
  } catch (error) {
    console.error('取消发布计划失败:', error)
    ElMessage.error('取消发布计划失败')
  }
}

// 暂停/继续发布
const togglePauseResume = async (plan: any) => {
  try {
    const action = plan.isPaused ? '继续' : '暂停'
    
    // 根据当前状态调用不同的后端API
    if (plan.isPaused) {
      const response = await apiService.continueDeployment(plan.id)
      console.log('继续部署计划成功:', response.data)
    } else {
      const response = await apiService.pauseDeployment(plan.id)
      console.log('暂停部署计划成功:', response.data)
    }
    
    ElMessage.success(`发布计划已${action}`)
    
    // 刷新发布计划列表以更新UI
    if (selectedNode.value) {
      const deploymentPlansDataResult = await loadServiceDeploymentPlans(selectedNode.value.name)
      if (deploymentPlansDataResult) {
        currentServiceDeploymentPlans.value = deploymentPlansDataResult
      }
    }
  } catch (error) {
    console.error('暂停/继续发布计划失败:', error)
    ElMessage.error('暂停/继续发布计划失败')
  }
}

// 回滚发布
const rollbackRelease = async (plan: any) => {
  try {
    // 调用后端API回滚部署计划
    const response = await apiService.rollbackDeployment(plan.id)
    
    console.log('回滚发布计划成功:', response.data)
    ElMessage.success('发布计划已回滚')
    
    // 刷新发布计划列表和服务详情
    if (selectedNode.value) {
      const deploymentPlansDataResult = await loadServiceDeploymentPlans(selectedNode.value.name)
      if (deploymentPlansDataResult) {
        currentServiceDeploymentPlans.value = deploymentPlansDataResult
      }
      
      // 刷新服务详情数据（饼图会自动更新）
      const serviceDetailResult = await loadServiceDetail(selectedNode.value.name)
      if (serviceDetailResult) {
        selectedNode.value = { ...serviceDetailResult, status: getNodeStatus(serviceDetailResult) }
      }
    }
  } catch (error) {
    console.error('回滚发布计划失败:', error)
    ElMessage.error('回滚发布计划失败')
  }
}

// 版本表格中的操作
const togglePauseResumeForVersion = async (version: any) => {
  try {
    if (version.isPaused) {
      // 调用后端API继续部署
      const response = await apiService.continueDeployment(version.deployId)
      console.log('继续部署成功:', response.data)
      ElMessage.success('继续部署成功')
    } else {
      // 调用后端API暂停部署
      const response = await apiService.pauseDeployment(version.deployId)
      console.log('暂停部署成功:', response.data)
      ElMessage.success('暂停部署成功')
    }
    
    // 重新加载服务详情数据以更新UI
    if (selectedNode.value) {
      const serviceDetailResult = await loadServiceDetail(selectedNode.value.name)
      if (serviceDetailResult) {
        selectedNode.value = { ...serviceDetailResult, status: getNodeStatus(serviceDetailResult) }
      }
      
      // 重新加载发布计划列表
      const deploymentPlansDataResult = await loadServiceDeploymentPlans(selectedNode.value.name)
      if (deploymentPlansDataResult) {
        currentServiceDeploymentPlans.value = deploymentPlansDataResult
      }
    }
  } catch (error) {
    console.error('操作失败:', error)
    ElMessage.error('操作失败')
  }
}

const rollbackVersion = async (version: any) => {
  try {
    // 调用后端API执行回滚操作
    const response = await apiService.rollbackDeployment(version.deployId)
    
    console.log('版本回滚成功:', response.data)
    
    // 重新加载服务详情数据（饼状图会自动更新）
    if (selectedNode.value) {
      const serviceDetailResult = await loadServiceDetail(selectedNode.value.name)
      if (serviceDetailResult) {
        selectedNode.value = { ...serviceDetailResult, status: getNodeStatus(serviceDetailResult) }
      }
    }
    
    // 重新加载可发布版本数据（下拉框会自动更新）
    if (selectedNode.value) {
      const availableVersionsResult = await loadServiceAvailableVersions(selectedNode.value.name)
      if (availableVersionsResult) {
        currentServiceAvailableVersions.value = availableVersionsResult
      }
    }
    
    // 重新加载服务指标数据
    if (selectedNode.value) {
      const metricsDataResult = await loadServiceMetrics(selectedNode.value.name)
      if (metricsDataResult) {
        currentServiceMetrics.value = metricsDataResult
      }
    }
    
    // 重新加载服务数据以更新拓扑图
    await loadServicesData()
    
    ElMessage.success('版本回滚成功')
    
  } catch (error) {
    console.error('回滚失败:', error)
    ElMessage.error('回滚失败')
  }
}

// 全局函数：更新服务告警状态（供告警界面调用）
const updateServiceAlertStatusFromAlerts = (serviceName: string, alertState: string) => {
  const status = updateServiceAlertStatus(serviceName, alertState)
  
  // 重新加载服务数据以更新拓扑图
  loadServicesData()
  
  console.log(`服务 ${serviceName} 告警状态已更新为: ${status}`)
  return status
}

// 将函数暴露到全局，供告警界面调用
;(window as any).updateServiceAlertStatusFromAlerts = updateServiceAlertStatusFromAlerts

// 初始化饼图
let pieChart: echarts.ECharts | null = null

// 指标图表实例
let latencyChart: echarts.ECharts | null = null
let trafficChart: echarts.ECharts | null = null
let errorsChart: echarts.ECharts | null = null
let saturationChart: echarts.ECharts | null = null

const initPieChart = () => {
  if (pieChartRef.value && selectedNode.value) {
    // 如果该DOM上已有实例，先释放
    const existing = echarts.getInstanceByDom(pieChartRef.value as unknown as HTMLDivElement)
    if (existing) {
      existing.dispose()
    }
    if (pieChart) {
      pieChart.dispose()
      pieChart = null
    }
    pieChart = echarts.init(pieChartRef.value)
    
    const option = {
      tooltip: {
        trigger: 'item',
        formatter: (params: any) => {
          const data = params.data
          let html = `<div style="padding: 10px;">
            <div style="font-weight: bold; margin-bottom: 8px;">灰度详情 · ${data.name}</div>
            <div style="display: flex; justify-content: space-between;">
              <span>占比</span>
              <span style="color: #666;">${data.value}%</span>
            </div>`
          
          // 显示开始时间和预估完成时间
          if (data.startTime && data.estimatedCompletionTime) {
            const startTime = new Date(data.startTime).toLocaleString('zh-CN', {
              year: 'numeric',
              month: '2-digit',
              day: '2-digit',
              hour: '2-digit',
              minute: '2-digit'
            })
            const estimatedTime = new Date(data.estimatedCompletionTime).toLocaleString('zh-CN', {
              year: 'numeric',
              month: '2-digit',
              day: '2-digit',
              hour: '2-digit',
              minute: '2-digit'
            })
            
            html += `<div style="margin-top: 6px; padding-top: 6px; border-top: 1px solid #eee;">
              <div style="margin-bottom: 4px;">
                <span style="color: #666;">开始时间:</span>
                <span style="margin-left: 8px; font-weight: 500;">${startTime}</span>
              </div>
              <div>
                <span style="color: #666;">预估完成:</span>
                <span style="margin-left: 8px; font-weight: 500;">${estimatedTime}</span>
              </div>
            </div>`
          }
          
          if (data.rolling) {
            html += `<div style="margin-top: 4px; color: #666;">
              发布持续时间 <b>${data.elapsedMin}</b> 分钟，预计剩余时间 <b>${data.remainingMin}</b> 分钟
            </div>`
            html += `<div style="margin-top: 4px;">ETA <span style="color: #666;">${data.eta}</span></div>`
          }
          
          // 根据版本的具体状态显示
          const versionStatus = data.anomalous ? 'abnormal' : (data.observing ? 'canary' : 'healthy')
          const statusText = data.anomalous ? '异常' : (data.observing ? '有异常点，AI正在观察和分析' : '正常')
          const statusColor = data.anomalous ? '#f43f5e' : (data.observing ? '#f59e0b' : '#10b981')
          html += `<div style="margin-top: 6px; padding-top: 6px; border-top: 1px solid #eee; color: ${statusColor}; font-size: 12px;">${statusText}</div>`
          html += '</div>'
          return html
        }
      },
      series: [{
        type: 'pie',
        radius: ['40%', '70%'],
        center: ['50%', '50%'],
        label: {
          show: true,
          position: 'outside',
          formatter: '{b}: {d}%',
          fontSize: 12,
          fontWeight: 'bold'
        },
        labelLine: {
          show: true,
          length: 15,
          length2: 10
        },
        data: selectedNode.value.versions.map((v: any, index: number) => {
          // 根据每个版本的具体状态确定颜色
          const versionStatus = v.anomalous ? 'abnormal' : (v.observing ? 'canary' : 'healthy')
          const statusMap: Record<string, string> = {
            healthy: "#10b981",    // 绿色
            abnormal: "#f43f5e",   // 红色
            canary: "#f59e0b"      // 黄色
          }
          
          return {
            name: v.label,
            value: v.value,
            ...v,
            itemStyle: {
              color: statusMap[versionStatus] || "#6b7280"
            }
          }
        }),
        emphasis: {
          itemStyle: {
            shadowBlur: 10,
            shadowOffsetX: 0,
            shadowColor: 'rgba(0, 0, 0, 0.5)'
          }
        }
      }]
    }
    
    pieChart.setOption(option)
    
    // 添加点击事件
    pieChart.on('click', (params: any) => {
      if (selectedNode.value) {
        selectedSlice.value = {
          nodeId: selectedNode.value.id,
          label: params.data.name
        }
        
        // 异步加载指标数据
        loadServiceMetricsData(selectedNode.value.name, params.data.name).then(metricsDataResult => {
          if (metricsDataResult) {
            metricsData.value = metricsDataResult
            // 数据加载完成后，重新初始化图表
            nextTick(() => {
              initMetricsCharts()
            })
          }
        })
        
        metricsDialogVisible.value = true
      }
    })
  }
}

// 监听 selectedNode 变化，重新初始化饼图
watch(() => selectedNode.value, () => {
  nextTick(() => {
    if (selectedNode.value) {
      initPieChart()
    }
  })
})

// 监听指标弹窗打开，初始化指标图表
watch(() => metricsDialogVisible.value, (newVal) => {
  if (newVal) {
    nextTick(() => {
      initMetricsCharts()
    })
  }
})



// 初始化指标图表
const initMetricsCharts = () => {
  // 延迟图表
  if (latencyChartRef.value && metricsData.value.latency) {
    latencyChart = echarts.init(latencyChartRef.value)
    const latencyValues = metricsData.value.latency.data.result[0]?.values || []
    const latencyData = latencyValues.map(([timestamp, value]) => [timestamp * 1000, parseFloat(value)])
    
    latencyChart.setOption({
      grid: { top: 8, right: 8, bottom: 0, left: 0 },
      xAxis: { type: 'time', show: false },
      yAxis: { type: 'value', show: false },
      series: [{
        type: 'line',
        data: latencyData,
        smooth: true,
        lineStyle: { width: 2 },
        symbol: 'none'
      }]
    })
  }
  
  // 流量图表
  if (trafficChartRef.value && metricsData.value.traffic) {
    trafficChart = echarts.init(trafficChartRef.value)
    const trafficValues = metricsData.value.traffic.data.result[0]?.values || []
    const trafficData = trafficValues.map(([timestamp, value]) => [timestamp * 1000, parseFloat(value)])
    
    trafficChart.setOption({
      grid: { top: 8, right: 8, bottom: 0, left: 0 },
      xAxis: { type: 'time', show: false },
      yAxis: { type: 'value', show: false },
      series: [{
        type: 'line',
        data: trafficData,
        smooth: true,
        lineStyle: { width: 2 },
        symbol: 'none'
      }]
    })
  }
  
  // 错误率图表
  if (errorsChartRef.value && metricsData.value.errors) {
    errorsChart = echarts.init(errorsChartRef.value)
    const errorsValues = metricsData.value.errors.data.result[0]?.values || []
    const errorsData = errorsValues.map(([timestamp, value]) => [timestamp * 1000, parseFloat(value)])
    
    errorsChart.setOption({
      grid: { top: 8, right: 8, bottom: 0, left: 0 },
      xAxis: { type: 'time', show: false },
      yAxis: { type: 'value', show: false },
      series: [{
        type: 'line',
        data: errorsData,
        smooth: true,
        lineStyle: { width: 2 },
        symbol: 'none'
      }]
    })
  }
  
  // 饱和度图表
  if (saturationChartRef.value && metricsData.value.saturation) {
    saturationChart = echarts.init(saturationChartRef.value)
    const saturationValues = metricsData.value.saturation.data.result[0]?.values || []
    const saturationData = saturationValues.map(([timestamp, value]) => [timestamp * 1000, parseFloat(value)])
    
    saturationChart.setOption({
      grid: { top: 8, right: 8, bottom: 0, left: 0 },
      xAxis: { type: 'time', show: false },
      yAxis: { type: 'value', show: false },
      series: [{
        type: 'line',
        data: saturationData,
        smooth: true,
        lineStyle: { width: 2 },
        symbol: 'none'
      }]
    })
  }
}

// 清理指标图表
const disposeMetricsCharts = () => {
  if (latencyChart) {
    latencyChart.dispose()
    latencyChart = null
  }
  if (trafficChart) {
    trafficChart.dispose()
    trafficChart = null
  }
  if (errorsChart) {
    errorsChart.dispose()
    errorsChart = null
  }
  if (saturationChart) {
    saturationChart.dispose()
    saturationChart = null
  }
}
</script>

<style scoped>
.home-container {
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

.nav-buttons {
  display: flex;
  gap: 12px;
  align-items: center;
}

.nav-btn {
  display: flex;
  align-items: center;
  gap: 4px;
}

.title {
  font-size: 24px;
  font-weight: 600;
  color: #2c3e50;
}

.subtitle {
  text-align: center;
  font-size: 18px;
  font-weight: 500;
  margin-top: -30px;
  margin-bottom: 12px;
  color: #2c3e50;
}

.topology-card {
  margin-bottom: 24px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}

.topology-container {
  position: relative;
  width: 100%;
  height: calc(100vh - 200px);
  min-height: 560px;
}

.loading-container, .error-container {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  color: #6b7280;
  font-size: 14px;
  z-index: 10;
}

.error-container {
  color: #f43f5e;
}

.error-container .el-button {
  margin-top: 8px;
}

.edges-svg {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}

.service-node {
  position: absolute;
  transform: translate(-50%, -50%);
  cursor: pointer;
}

.node-circle {
  position: relative;
  width: 64px;
  height: 64px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  border: 2px solid rgba(255, 255, 255, 0.2);
}

.node-name {
  color: white;
  font-size: 14px;
  font-weight: 500;
  user-select: none;
}

.rolling-indicator {
  position: absolute;
  top: -4px;
  right: -4px;
  width: 24px;
  height: 24px;
  background: white;
  border: 1px solid #e2e8f0;
  border-radius: 50%;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.rolling-indicator svg {
  width: 100%;
  height: 100%;
}

.legend {
  position: absolute;
  bottom: 60px;
  right: 12px;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(8px);
  border-radius: 8px;
  border: 1px solid #e2e8f0;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  padding: 12px;
  font-size: 12px;
  color: #475569;
}

.legend-title {
  font-weight: 500;
  color: #1e293b;
  margin-bottom: 8px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.legend-divider {
  height: 1px;
  background: #e2e8f0;
  margin: 8px 0;
}

.rolling-example {
  width: 16px;
  height: 16px;
  background: white;
  border: 1px solid #e2e8f0;
  border-radius: 50%;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.rolling-example svg {
  width: 100%;
  height: 100%;
}

.service-detail {
  padding: 16px 0;
  width: 100%;
  height: 100%;
}

.detail-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.detail-desc {
  color: #6b7280;
  margin-bottom: 24px;
  font-size: 14px;
}

.detail-content {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
  width: 100%;
  height: 100%;
}

.pie-chart-container {
  height: 320px;
  width: 100%;
}

.pie-chart {
  width: 100%;
  height: 100%;
}

.version-info {
  display: flex;
  flex-direction: column;
  gap: 16px;
  width: 100%;
}

.version-title {
  font-size: 14px;
  color: #6b7280;
}

.version-table {
  width: 100%;
}

.release-controls {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  width: 100%;
}

.release-plans {
  max-height: 256px;
  overflow-y: auto;
}

.plans-header {
  font-weight: 500;
  font-size: 14px;
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid #e5e7eb;
}

.plans-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.plan-item {
  padding: 12px;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  transition: background-color 0.2s;
}

.plan-item:hover {
  background-color: #f9fafb;
}

.plan-content {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
}

.plan-info {
  flex: 1;
}

.plan-header {
  margin-bottom: 8px;
}

.plan-version {
  font-weight: 500;
  font-size: 14px;
}

.plan-details {
  font-size: 12px;
  color: #6b7280;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.plan-actions {
  display: flex;
  justify-content: flex-start;
  align-items: flex-end;
  min-width: 100px;
  padding-left: 10px;
}

.action-column {
  display: flex;
  flex-direction: row;
  gap: 8px;
  align-items: center;
}

/* 表格操作样式 */
.table-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: flex-end;
  padding-right: 10px;
}

.action-link {
  cursor: pointer;
  font-size: 12px;
  text-decoration: none;
  padding: 2px 4px;
  border-radius: 2px;
  transition: all 0.2s;
}

.action-link:hover {
  opacity: 0.8;
}

/* 暂停按钮 - 黄色 */
.action-link.pause {
  color: #e6a23c;
}

.action-link.pause:hover {
  background-color: #fdf6ec;
}

/* 继续按钮 - 绿色 */
.action-link.continue {
  color: #67c23a;
}

.action-link.continue:hover {
  background-color: #f0f9ff;
}

/* 编辑按钮 - 黑色 */
.action-link.edit {
  color: #303133;
}

.action-link.edit:hover {
  background-color: #f5f7fa;
}

/* 取消按钮 - 红色 */
.action-link.cancel {
  color: #f56c6c;
}

.action-link.cancel:hover {
  background-color: #fef0f0;
}

/* 回滚按钮 - 蓝色（保持原有颜色） */
.action-link.rollback {
  color: #409eff;
}

.action-link.rollback:hover {
  background-color: #ecf5ff;
}

.action-link:not(:last-child)::after {
  content: '|';
  margin-left: 8px;
  color: #dcdfe6;
}

.completed-text {
  color: #67c23a;
  font-size: 12px;
  font-weight: 500;
}

.no-plans {
  text-align: center;
  color: #6b7280;
  font-size: 14px;
  padding: 16px;
}

.no-data {
  text-align: center;
  color: #6b7280;
  font-size: 14px;
  padding: 20px;
}

/* 指标弹窗样式 */
.metrics-dialog-content {
  padding: 0;
}

.metrics-header {
  margin-bottom: 24px;
  text-align: center;
}

.metrics-header h3 {
  margin: 0 0 8px 0;
  font-size: 20px;
  font-weight: 600;
  color: #1f2937;
}

.metrics-subtitle {
  margin: 0;
  color: #6b7280;
  font-size: 14px;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 24px;
}

.metric-panel {
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 20px;
  transition: all 0.2s ease;
}

.metric-panel:hover {
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
  border-color: #d1d5db;
}

.metric-header {
  margin-bottom: 16px;
}

.metric-header h4 {
  margin: 0 0 4px 0;
  font-size: 16px;
  font-weight: 600;
  color: #1f2937;
}

.metric-note {
  margin: 0;
  font-size: 12px;
  color: #6b7280;
}

.metric-value {
  display: flex;
  align-items: baseline;
  margin-bottom: 16px;
}

.metric-value .value {
  font-size: 32px;
  font-weight: 700;
  color: #1f2937;
  margin-right: 8px;
}

.metric-value .unit {
  font-size: 16px;
  color: #6b7280;
  font-weight: 500;
}

.metric-chart {
  height: 160px;
  width: 100%;
}

@media (max-width: 768px) {
  .metrics-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .detail-content {
    grid-template-columns: 1fr;
  }
  
  .release-controls {
    flex-direction: column;
    align-items: stretch;
  }
}

/* 编辑部署对话框样式 */
.edit-deployment-form {
  padding: 20px 0;
}

.form-item {
  margin-bottom: 20px;
}

.form-label {
  display: block;
  font-size: 14px;
  font-weight: 500;
  color: #374151;
  margin-bottom: 8px;
}

.form-label .required {
  color: #ef4444;
  margin-left: 4px;
}

.form-value {
  font-size: 14px;
  color: #6b7280;
  padding: 8px 12px;
  background-color: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  min-height: 20px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

</style>