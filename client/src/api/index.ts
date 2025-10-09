import axios from 'axios'

// 创建 axios 实例
const api = axios.create({
  // baseURL: (import.meta as any).env?.VITE_API_BASE_URL || 'http://10.210.10.33:8080',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 请求拦截器
api.interceptors.request.use(
  (config) => {
    console.log('发送请求:', config.method?.toUpperCase(), config.url)
    return config
  },
  (error) => {
    console.error('请求错误:', error)
    return Promise.reject(error)
  }
)

// 响应拦截器
api.interceptors.response.use(
  (response) => {
    console.log('收到响应:', response.status, response.config.url)
    return response
  },
  (error) => {
    console.error('响应错误:', error.response?.status, error.message)
    return Promise.reject(error)
  }
)

// API 接口定义
export const apiService = {
  // 获取服务列表
  getServices: () => {
    return api.get('/v1/services')
  },

  // 获取服务活跃版本
  getServiceActiveVersions: (serviceName: string) => {
    return api.get(`/v1/services/${serviceName}/activeVersions`)
  },

  // 获取服务指标统计
  getServiceMetrics: (serviceName: string) => {
    return api.get(`/v1/services/${serviceName}/metricStats`)
  },

  // 获取服务可发布版本列表
  getServiceAvailableVersions: (serviceName: string) => {
    return api.get(`/v1/services/${serviceName}/availableVersions?type=unrelease`)
  },

  // 获取服务发布计划列表
  getServiceDeploymentPlans: (serviceName: string) => {
    return api.get(`/v1/deployments?type=schedule&service=${serviceName}`)
  },

  // 获取服务指标数据
  getServiceMetricsData: (serviceName: string, metricName: string, version: string) => {
    const now = new Date()
    const thirtyMinutesAgo = new Date(now.getTime() - 30 * 60 * 1000) // 30分钟前

    const start = thirtyMinutesAgo.toISOString()
    const end = now.toISOString()
    const granule = '5m' // 写死，每5分钟一个数据点

    return api.get(`/v1/metrics/${serviceName}/${metricName}?version=${version}&start=${start}&end=${end}&granule=${granule}`)
  },

  // 取消部署计划
  cancelDeployment: (deployID: string) => {
    return api.delete(`/v1/deployments/${deployID}`)
  },

  // 暂停部署计划
  pauseDeployment: (deployID: string) => {
    return api.post(`/v1/deployments/${deployID}/pause`)
  },

  // 继续部署计划
  continueDeployment: (deployID: string) => {
    return api.post(`/v1/deployments/${deployID}/continue`)
  },

  // 回滚部署计划
  rollbackDeployment: (deployID: string) => {
    return api.post(`/v1/deployments/${deployID}/rollback`)
  },

  // 获取部署变更记录
  getDeploymentChangelog: (start?: string, limit?: number) => {
    const params: any = {}
    if (start) params.start = start
    if (limit) params.limit = limit
    return api.get('/v1/changelog/deployment', { params })
  },

  // 获取告警规则变更记录
  getAlertRuleChangelog: (start?: string, limit?: number) => {
    const params: any = {}
    if (start) params.start = start
    if (limit) params.limit = limit
    return api.get('/v1/changelog/alertrules', { params })
  },

  // 获取告警列表
  getAlerts: (start?: string, limit?: number, state?: string) => {
    const params: any = {}
    if (start) params.start = start
    if (limit) params.limit = limit
    if (state) params.state = state
    return api.get('/v1/issues', { params })
  },

  // 获取告警详情
  getAlertDetail: (issueID: string) => {
    return api.get(`/v1/issues/${issueID}`)
  },

  // 更新部署计划
  updateDeployment: (deployID: string, data: {version?: string, scheduleTime?: string}) => {
    return api.post(`/v1/deployments/${deployID}`, data)
  },

  // 创建部署计划
  createDeployment: (data: {service: string, version: string, scheduleTime?: string}) => {
    return api.post('/v1/deployments', data)
  }
}

export default api
