// Mock数据 - 服务拓扑数据
// 根据我们商议的方案：使用dependencies数组记录依赖关系

export interface ServiceItem {
  name: string
  deployState: 'unrelease' | 'deploying' | 'stop' | 'rollback' | 'completed'
  health: 'Normal' | 'Warning' | 'Error'
  deps: string[]
}

export interface ServicesResponse {
  items: ServiceItem[]
}

// Mock数据 - 根据当前写死的数据转换而来
export const mockServicesData: ServicesResponse = {
  items: [
    {
      name: "s3",
      deployState: "completed",
      health: "Normal",
      deps: [] // s3是根节点，无依赖
    },
    {
      name: "stg",
      deployState: "completed",
      health: "Normal",
      deps: ["s3"] // stg依赖s3
    },
    {
      name: "meta",
      deployState: "completed",
      health: "Normal",
      deps: ["s3"] // meta依赖s3
    },
    {
      name: "mq",
      deployState: "completed",
      health: "Normal",
      deps: ["s3"] // mq依赖s3
    },
    {
      name: "worker",
      deployState: "completed",
      health: "Normal",
      deps: ["mq"] // worker依赖mq
    },
    {
      name: "mongodb",
      deployState: "completed",
      health: "Normal",
      deps: ["meta"] // mongodb依赖meta
    }
  ]
}

// Mock数据 - 服务版本详情（用于弹窗显示）
export interface ServiceVersion {
  label: string
  value: number
  eta: string
  anomalous: boolean
  observing: boolean
  rolling?: boolean
  elapsedMin?: number
  remainingMin?: number
}

export interface ServiceDetail {
  name: string
  deployState: 'unrelease' | 'deploying' | 'stop' | 'rollback' | 'completed'
  health: 'Normal' | 'Warning' | 'Error'
  deps: string[]
  versions: ServiceVersion[]
}

export const mockServiceDetails: Record<string, ServiceDetail> = {
  "s3": {
    name: "s3",
    deployState: "completed",
    health: "Normal",
    deps: [],
    versions: [
      { label: "v1.0.0", value: 60, eta: "~ 2h 30m", anomalous: false, observing: false },
      { label: "v1.0.1", value: 30, eta: "~ 1h 10m", anomalous: false, observing: false },
      { label: "v1.0.3", value: 10, eta: "~ 40m", anomalous: false, observing: false }
    ]
  },
  "stg": {
    name: "stg",
    deployState: "completed",
    health: "Normal",
    deps: ["s3"],
    versions: [
      { label: "v1.0.0", value: 70, eta: "~ 3h 00m", anomalous: false, observing: false },
      { label: "v1.0.2", value: 30, eta: "~ 30m", anomalous: false, observing: false }
    ]
  },
  "meta": {
    name: "meta",
    deployState: "completed",
    health: "Normal",
    deps: ["s3"],
    versions: [
      { label: "v1.0.3", value: 100, eta: "~ 25m", anomalous: false, observing: false }
    ]
  },
  "mq": {
    name: "mq",
    deployState: "completed",
    health: "Normal",
    deps: ["s3"],
    versions: [
      { label: "v1.0.1", value: 100, eta: "~ 50m", anomalous: false, observing: false }
    ]
  },
  "worker": {
    name: "worker",
    deployState: "completed",
    health: "Normal",
    deps: ["mq"],
    versions: [
      { label: "v1.0.1", value: 100, eta: "~ 20m", anomalous: false, observing: false }
    ]
  },
  "mongodb": {
    name: "mongodb",
    deployState: "completed",
    health: "Normal",
    deps: ["meta"],
    versions: [
      { label: "v1.0.1", value: 80, eta: "~ 1h 10m", anomalous: false, observing: false },
      { label: "v1.0.2", value: 20, eta: "~ 45m", anomalous: false, observing: false }
    ]
  }
}

// 可发布版本数据结构 - 匹配后端API返回格式
export interface AvailableVersion {
  version: string
  createTime: string
}

export interface AvailableVersionsResponse {
  items: AvailableVersion[]
}

// Mock数据 - 可发布版本列表
export const mockAvailableVersions: Record<string, AvailableVersionsResponse> = {
  "s3": {
    items: [
      { version: "v1.0.4", createTime: "2024-01-01T03:00:00Z" },
      { version: "v1.0.5", createTime: "2024-01-02T03:00:00Z" },
      { version: "v1.0.6", createTime: "2024-01-03T03:00:00Z" },
      { version: "v1.0.7", createTime: "2024-01-04T03:00:00Z" },
      { version: "v1.0.8", createTime: "2024-01-05T03:00:00Z" }
    ]
  },
  "stg": {
    items: [
      { version: "v1.0.3", createTime: "2024-01-01T02:00:00Z" },
      { version: "v1.0.4", createTime: "2024-01-02T02:00:00Z" },
      { version: "v1.0.5", createTime: "2024-01-03T02:00:00Z" },
      { version: "v1.0.6", createTime: "2024-01-04T02:00:00Z" }
    ]
  },
  "meta": {
    items: [
      { version: "v1.0.4", createTime: "2024-01-01T01:00:00Z" },
      { version: "v1.0.5", createTime: "2024-01-02T01:00:00Z" },
      { version: "v1.0.6", createTime: "2024-01-03T01:00:00Z" }
    ]
  },
  "mq": {
    items: [
      { version: "v1.0.2", createTime: "2024-01-01T00:30:00Z" },
      { version: "v1.0.3", createTime: "2024-01-02T00:30:00Z" },
      { version: "v1.0.4", createTime: "2024-01-03T00:30:00Z" }
    ]
  },
  "worker": {
    items: [
      { version: "v1.0.2", createTime: "2024-01-01T00:15:00Z" },
      { version: "v1.0.3", createTime: "2024-01-02T00:15:00Z" },
      { version: "v1.0.4", createTime: "2024-01-03T00:15:00Z" },
      { version: "v1.0.5", createTime: "2024-01-04T00:15:00Z" }
    ]
  },
  "mongodb": {
    items: [
      { version: "v1.0.2", createTime: "2024-01-01T00:10:00Z" },
      { version: "v1.0.3", createTime: "2024-01-02T00:10:00Z" }
    ]
  }
}

// 兼容性：保留原有的版本选项格式（用于全局版本选择）
export const mockVersionOptions = [
  { label: 'v1.0.4', value: 'v1.0.4' },
  { label: 'v1.0.5', value: 'v1.0.5' },
  { label: 'v1.0.6', value: 'v1.0.6' },
  { label: 'v1.0.7', value: 'v1.0.7' }
]

// 发布计划数据结构 - 匹配后端API返回格式
export interface DeploymentPlan {
  id: string
  service: string
  version: string
  status: 'Schedule' | 'InDeployment' | 'Finished'
  scheduleTime?: string
  finishTime?: string
  isPaused?: boolean
}

export interface DeploymentPlansResponse {
  items: DeploymentPlan[]
}

// 告警规则变更记录数据结构
export interface AlertRuleChangeValue {
  name: string
  old: string
  new: string
}

export interface AlertRuleChangeItem {
  name: string
  editTime: string
  scope: string
  values: AlertRuleChangeValue[]
  reason: string
}

export interface AlertRuleChangelogResponse {
  items: AlertRuleChangeItem[]
  next?: string
}

// Mock数据 - 发布计划（按服务分组）
export const mockDeploymentPlans: Record<string, DeploymentPlansResponse> = {
  "s3": {
    items: [
      {
        id: "1001",
        service: "s3",
        version: "v1.0.4",
        status: "InDeployment",
        scheduleTime: "2024-01-15T14:00:00Z",
        isPaused: false
      },
      {
        id: "1002",
        service: "s3",
        version: "v1.0.5",
        status: "InDeployment",
        scheduleTime: "2024-01-16T10:00:00Z",
        isPaused: true
      },
      {
        id: "1003",
        service: "s3",
        version: "v1.0.3",
        status: "Finished",
        scheduleTime: "2024-01-14T09:00:00Z",
        finishTime: "2024-01-14T18:00:00Z"
      }
    ]
  },
  "stg": {
    items: [
      {
        id: "2001",
        service: "stg",
        version: "v1.0.1",
        status: "InDeployment",
        scheduleTime: "",
        isPaused: false
      },
      {
        id: "2002",
        service: "stg",
        version: "v1.0.2",
        status: "InDeployment",
        scheduleTime: "2024-01-03T05:00:00Z",
        isPaused: false
      }
    ]
  },
  "meta": {
    items: [
      {
        id: "3001",
        service: "meta",
        version: "v1.0.4",
        status: "InDeployment",
        scheduleTime: "2024-01-17T09:00:00Z"
      },
      {
        id: "3002",
        service: "meta",
        version: "v1.0.5",
        status: "Finished",
        finishTime: "2024-01-16T15:30:00Z"
      }
    ]
  },
  "mq": {
    items: [
      {
        id: "4001",
        service: "mq",
        version: "v1.0.2",
        status: "InDeployment",
        scheduleTime: "2024-01-18T12:00:00Z"
      },
      {
        id: "4002",
        service: "mq",
        version: "v1.0.3",
        status: "Finished",
        finishTime: "2024-01-17T20:00:00Z"
      }
    ]
  },
  "worker": {
    items: [
      {
        id: "5001",
        service: "worker",
        version: "v1.0.2",
        status: "InDeployment",
        scheduleTime: "2024-01-19T08:00:00Z"
      },
      {
        id: "5002",
        service: "worker",
        version: "v1.0.3",
        status: "Finished",
        finishTime: "2024-01-18T16:00:00Z"
      },
      {
        id: "5003",
        service: "worker",
        version: "v1.0.4",
        status: "InDeployment",
        scheduleTime: ""
      }
    ]
  },
  "mongodb": {
    items: [
      {
        id: "6001",
        service: "mongodb",
        version: "v1.0.2",
        status: "InDeployment",
        scheduleTime: "2024-01-20T14:00:00Z"
      },
      {
        id: "6002",
        service: "mongodb",
        version: "v1.0.3",
        status: "Finished",
        finishTime: "2024-01-19T11:00:00Z"
      }
    ]
  }
}

// 兼容性：保留原有的发布计划格式（用于全局发布计划）
export const mockScheduledReleases = [
  {
    id: 'release-1',
    version: 'v1.0.4',
    startTime: '2024-01-15 14:00:00',
    creator: '张三'
  },
  {
    id: 'release-2',
    version: 'v1.0.5',
    startTime: '2024-01-15 16:00:00',
    creator: '李四'
  }
]

// 指标API数据结构 - 匹配Prometheus query_range格式
export interface MetricData {
  metric: {
    __name__: string
    job?: string
    instance?: string
    version?: string
    service?: string
  }
  values: Array<[number, string]> // [timestamp, value]
}

export interface MetricsResponse {
  status: 'success' | 'error'
  data: {
    resultType: 'matrix' | 'vector' | 'scalar' | 'string'
    result: MetricData[]
  }
}

// 部署变更记录数据类型定义
export interface DeploymentChangelogItem {
  service: string
  version: string
  startTime: string
  endTime?: string
  instances: number
  totalInstances: number
  health: 'Normal' | 'Warning' | 'Error'
}

export interface DeploymentChangelogResponse {
  items: DeploymentChangelogItem[]
  next?: string
}

// 扩展的部署变更记录数据类型，包含详细的分批次信息
export interface DetailedDeploymentChangelogItem extends DeploymentChangelogItem {
  batches?: {
    name: string
    status: '正常' | '异常' | '进行中'
    start: string
    end: string
    anomaly?: string
    moduleRecords?: {
      id: string
      module: string
      action: string
      timestamp: string
      status: '成功' | '失败' | '告警' | '回滚'
      details?: string
      eventData?: any
    }[]
  }[]
}

// Mock数据 - 部署变更记录（包含详细的分批次信息）
export const mockDeploymentChangelog: { items: DetailedDeploymentChangelogItem[], next?: string } = {
  items: [
    {
      service: "stg",
      version: "v1.0.3",
      startTime: "2024-01-15T14:00:00Z",
      endTime: "2024-01-15T16:00:00Z",
      instances: 50,
      totalInstances: 100,
      health: "Warning",
      batches: [
        { 
          name: '第一批', 
          status: '正常', 
          start: '2024-01-15 14:00:00', 
          end: '2024-01-15 14:10:00' 
        },
        { 
          name: '第二批', 
          status: '正常', 
          start: '2024-01-15 14:20:00', 
          end: '2024-01-15 14:35:00' 
        },
        {
          name: '第三批',
          status: '异常',
          start: '2024-01-15 14:50:00',
          end: '2024-01-15 15:10:00',
          anomaly: 'Stg服务指标异常。',
          moduleRecords: [
            {
              id: 'event-1',
              module: '发布系统',
              action: '发布到节点',
              timestamp: '2024-01-15 14:50:00',
              status: '成功',
              details: '发布到 bj1-node-002, qn1-node-002 节点'
            },
            {
              id: 'event-2',
              module: '发布系统',
              action: '部署新版本',
              timestamp: '2024-01-15 14:50:15',
              status: '成功',
              eventData: {
                deployment: {
                  service: "Stg服务v1.0.3",
                  environment: "生产环境",
                  healthCheck: "通过",
                  loadBalancer: "已更新路由规则",
                  trafficStatus: "新版本开始接收流量"
                }
              }
            },
            {
              id: 'event-3',
              module: '监控告警系统',
              action: '灰度发布监控',
              timestamp: '2024-01-15 14:55:20',
              status: '告警',
              eventData: {
                monitoringAlert: {
                  service: "Stg服务",
                  phase: "灰度过程中",
                  issue: "延迟异常"
                },
                metrics: {
                  p95Latency: {
                    before: "120ms",
                    after: "450ms",
                    increase: "275%"
                  },
                  errorRate: {
                    before: "0.1%",
                    after: "2.3%",
                    increase: "2200%"
                  }
                }
              }
            },
            {
              id: 'event-4',
              module: '发布系统',
              action: '自动回滚',
              timestamp: '2024-01-15 15:05:45',
              status: '回滚',
              eventData: {
                rollbackTrigger: {
                  trigger: "AI检测到异常后自动触发回滚流程",
                  aiJudgment: "基于延迟和错误率指标，AI判断当前版本存在严重问题"
                }
              }
            }
          ]
        }
      ]
    },
    {
      service: "stg", 
      version: "v1.0.2",
      startTime: "2024-01-14T10:00:00Z",
      endTime: "2024-01-14T12:00:00Z",
      instances: 30,
      totalInstances: 100,
      health: "Normal",
      batches: [
        { 
          name: '第一批', 
          status: '正常', 
          start: '2024-01-14 10:00:00', 
          end: '2024-01-14 10:15:00' 
        },
        { 
          name: '第二批', 
          status: '正常', 
          start: '2024-01-14 10:15:00', 
          end: '2024-01-14 10:30:00' 
        },
        { 
          name: '第三批', 
          status: '正常', 
          start: '2024-01-14 10:30:00', 
          end: '2024-01-14 10:45:00' 
        }
      ]
    },
    {
      service: "stg",
      version: "v1.0.1", 
      startTime: "2024-01-13T08:00:00Z",
      endTime: "2024-01-13T10:00:00Z",
      instances: 100,
      totalInstances: 100,
      health: "Normal",
      batches: [
        { 
          name: '第一批', 
          status: '正常', 
          start: '2024-01-13 08:00:00', 
          end: '2024-01-13 08:20:00' 
        },
        { 
          name: '第二批', 
          status: '正常', 
          start: '2024-01-13 08:20:00', 
          end: '2024-01-13 08:40:00' 
        },
        { 
          name: '第三批', 
          status: '正常', 
          start: '2024-01-13 08:40:00', 
          end: '2024-01-13 09:00:00' 
        }
      ]
    },
    {
      service: "s3",
      version: "v1.0.4",
      startTime: "2024-01-15T14:00:00Z",
      instances: 25,
      totalInstances: 50,
      health: "Normal",
      batches: [
        { 
          name: '第一批', 
          status: '正常', 
          start: '2024-01-15 14:00:00', 
          end: '2024-01-15 14:10:00' 
        },
        { 
          name: '第二批', 
          status: '进行中', 
          start: '2024-01-15 14:10:00', 
          end: '-' 
        }
      ]
    },
    {
      service: "s3",
      version: "v1.0.3",
      startTime: "2024-01-14T18:00:00Z",
      endTime: "2024-01-14T20:00:00Z", 
      instances: 50,
      totalInstances: 50,
      health: "Normal",
      batches: [
        { 
          name: '第一批', 
          status: '正常', 
          start: '2024-01-14 18:00:00', 
          end: '2024-01-14 18:15:00' 
        },
        { 
          name: '第二批', 
          status: '正常', 
          start: '2024-01-14 18:15:00', 
          end: '2024-01-14 18:30:00' 
        },
        { 
          name: '第三批', 
          status: '正常', 
          start: '2024-01-14 18:30:00', 
          end: '2024-01-14 18:45:00' 
        }
      ]
    }
  ],
  next: "2024-01-15T14:00:00Z"
}

// Mock指标数据 - 为每个服务的每个指标生成数据
export const mockMetricsData: Record<string, Record<string, MetricsResponse>> = {
  "s3": {
    "latency": {
      status: "success",
      data: {
        resultType: "matrix",
        result: [
          {
            metric: {
              __name__: "latency",
              service: "s3",
              version: "v1.0.1",
              instance: "s3-01"
            },
            values: [
              [1704268800, "45"], // 2024-01-03 06:00:00
              [1704269100, "42"], // 2024-01-03 06:05:00
              [1704269400, "48"], // 2024-01-03 06:10:00
              [1704269700, "41"], // 2024-01-03 06:15:00
              [1704270000, "44"], // 2024-01-03 06:20:00
              [1704270300, "46"]  // 2024-01-03 06:25:00
            ]
          }
        ]
      }
    },
    "traffic": {
      status: "success",
      data: {
        resultType: "matrix",
        result: [
          {
            metric: {
              __name__: "traffic",
              service: "s3",
              version: "v1.0.1",
              instance: "s3-01"
            },
            values: [
              [1704268800, "1200"],
              [1704269100, "1180"],
              [1704269400, "1250"],
              [1704269700, "1190"],
              [1704270000, "1210"],
              [1704270300, "1230"]
            ]
          }
        ]
      }
    },
    "errors": {
      status: "success",
      data: {
        resultType: "matrix",
        result: [
          {
            metric: {
              __name__: "errors",
              service: "s3",
              version: "v1.0.1",
              instance: "s3-01"
            },
            values: [
              [1704268800, "2.5"],
              [1704269100, "2.1"],
              [1704269400, "2.8"],
              [1704269700, "2.3"],
              [1704270000, "2.4"],
              [1704270300, "2.6"]
            ]
          }
        ]
      }
    },
    "saturation": {
      status: "success",
      data: {
        resultType: "matrix",
        result: [
          {
            metric: {
              __name__: "saturation",
              service: "s3",
              version: "v1.0.1",
              instance: "s3-01"
            },
            values: [
              [1704268800, "75"],
              [1704269100, "72"],
              [1704269400, "78"],
              [1704269700, "74"],
              [1704270000, "76"],
              [1704270300, "77"]
            ]
          }
        ]
      }
    }
  },
  "stg": {
    "latency": {
      status: "success",
      data: {
        resultType: "matrix",
        result: [
          {
            metric: {
              __name__: "latency",
              service: "stg",
              version: "v1.0.1",
              instance: "stg-01"
            },
            values: [
              [1704268800, "38"],
              [1704269100, "35"],
              [1704269400, "41"],
              [1704269700, "36"],
              [1704270000, "39"],
              [1704270300, "40"]
            ]
          }
        ]
      }
    },
    "traffic": {
      status: "success",
      data: {
        resultType: "matrix",
        result: [
          {
            metric: {
              __name__: "traffic",
              service: "stg",
              version: "v1.0.1",
              instance: "stg-01"
            },
            values: [
              [1704268800, "950"],
              [1704269100, "920"],
              [1704269400, "980"],
              [1704269700, "940"],
              [1704270000, "960"],
              [1704270300, "970"]
            ]
          }
        ]
      }
    },
    "errors": {
      status: "success",
      data: {
        resultType: "matrix",
        result: [
          {
            metric: {
              __name__: "errors",
              service: "stg",
              version: "v1.0.1",
              instance: "stg-01"
            },
            values: [
              [1704268800, "1.8"],
              [1704269100, "1.5"],
              [1704269400, "2.1"],
              [1704269700, "1.7"],
              [1704270000, "1.9"],
              [1704270300, "2.0"]
            ]
          }
        ]
      }
    },
    "saturation": {
      status: "success",
      data: {
        resultType: "matrix",
        result: [
          {
            metric: {
              __name__: "saturation",
              service: "stg",
              version: "v1.0.1",
              instance: "stg-01"
            },
            values: [
              [1704268800, "68"],
              [1704269100, "65"],
              [1704269400, "71"],
              [1704269700, "67"],
              [1704270000, "69"],
              [1704270300, "70"]
            ]
          }
        ]
      }
    }
  },
  "meta": {
    "latency": {
      status: "success",
      data: {
        resultType: "matrix",
        result: [
          {
            metric: {
              __name__: "latency",
              service: "meta",
              version: "v1.0.1",
              instance: "meta-01"
            },
            values: [
              [1704268800, "52"],
              [1704269100, "49"],
              [1704269400, "55"],
              [1704269700, "50"],
              [1704270000, "53"],
              [1704270300, "54"]
            ]
          }
        ]
      }
    },
    "traffic": {
      status: "success",
      data: {
        resultType: "matrix",
        result: [
          {
            metric: {
              __name__: "traffic",
              service: "meta",
              version: "v1.0.1",
              instance: "meta-01"
            },
            values: [
              [1704268800, "800"],
              [1704269100, "780"],
              [1704269400, "820"],
              [1704269700, "790"],
              [1704270000, "810"],
              [1704270300, "815"]
            ]
          }
        ]
      }
    },
    "errors": {
      status: "success",
      data: {
        resultType: "matrix",
        result: [
          {
            metric: {
              __name__: "errors",
              service: "meta",
              version: "v1.0.1",
              instance: "meta-01"
            },
            values: [
              [1704268800, "3.2"],
              [1704269100, "2.9"],
              [1704269400, "3.5"],
              [1704269700, "3.0"],
              [1704270000, "3.3"],
              [1704270300, "3.4"]
            ]
          }
        ]
      }
    },
    "saturation": {
      status: "success",
      data: {
        resultType: "matrix",
        result: [
          {
            metric: {
              __name__: "saturation",
              service: "meta",
              version: "v1.0.1",
              instance: "meta-01"
            },
            values: [
              [1704268800, "82"],
              [1704269100, "79"],
              [1704269400, "85"],
              [1704269700, "81"],
              [1704270000, "83"],
              [1704270300, "84"]
            ]
          }
        ]
      }
    }
  }
}

// 服务活跃版本数据结构 - 匹配后端API返回格式
export interface ServiceActiveVersion {
  version: string
  deployID: string
  startTime: string
  estimatedCompletionTime: string
  instances: number
  health: 'Normal' | 'Warning' | 'Error'
}

export interface ServiceActiveVersionsResponse {
  items: ServiceActiveVersion[]
}

// Mock数据 - 服务活跃版本数据
export const mockServiceActiveVersions: Record<string, ServiceActiveVersionsResponse> = {
  "s3": {
    items: [
      {
        version: "v1.0.0",
        deployID: "deploy-001",
        startTime: "2024-01-01T00:00:00Z",
        estimatedCompletionTime: "2024-01-01T02:30:00Z",
        instances: 60,
        health: "Normal"
      },
      {
        version: "v1.0.1",
        deployID: "deploy-002",
        startTime: "2024-01-01T01:00:00Z",
        estimatedCompletionTime: "2024-01-01T02:10:00Z",
        instances: 30,
        health: "Normal"
      },
      {
        version: "v1.0.3",
        deployID: "deploy-003",
        startTime: "2024-01-01T01:30:00Z",
        estimatedCompletionTime: "2024-01-01T02:10:00Z",
        instances: 10,
        health: "Normal"
      }
    ]
  },
  "stg": {
    items: [
      {
        version: "v1.0.0",
        deployID: "deploy-004",
        startTime: "2024-01-01T00:00:00Z",
        estimatedCompletionTime: "2024-01-01T03:00:00Z",
        instances: 70,
        health: "Normal"
      },
      {
        version: "v1.0.2",
        deployID: "deploy-005",
        startTime: "2024-01-01T01:15:00Z",
        estimatedCompletionTime: "2024-01-01T01:45:00Z",
        instances: 30,
        health: "Normal"
      }
    ]
  },
  "meta": {
    items: [
      {
        version: "v1.0.3",
        deployID: "deploy-006",
        startTime: "2024-01-01T00:00:00Z",
        estimatedCompletionTime: "2024-01-01T00:25:00Z",
        instances: 100,
        health: "Normal"
      }
    ]
  },
  "mq": {
    items: [
      {
        version: "v1.0.1",
        deployID: "deploy-007",
        startTime: "2024-01-01T00:00:00Z",
        estimatedCompletionTime: "2024-01-01T00:50:00Z",
        instances: 100,
        health: "Normal"
      }
    ]
  },
  "worker": {
    items: [
      {
        version: "v1.0.1",
        deployID: "deploy-008",
        startTime: "2024-01-01T00:00:00Z",
        estimatedCompletionTime: "2024-01-01T00:20:00Z",
        instances: 100,
        health: "Normal"
      }
    ]
  },
  "mongodb": {
    items: [
      {
        version: "v1.0.1",
        deployID: "deploy-009",
        startTime: "2024-01-01T00:00:00Z",
        estimatedCompletionTime: "2024-01-01T01:10:00Z",
        instances: 80,
        health: "Normal"
      },
      {
        version: "v1.0.2",
        deployID: "deploy-010",
        startTime: "2024-01-01T00:30:00Z",
        estimatedCompletionTime: "2024-01-01T01:15:00Z",
        instances: 20,
        health: "Normal"
      }
    ]
  }
}

// 指标数据结构 - 匹配后端返回格式
export interface Metric {
  name: string
  value: number
}

export interface ServiceMetricsSummary {
  metrics: Metric[]
}

export interface ServiceMetricsItem {
  version: string
  metrics: Metric[]
}

export interface ServiceMetricsResponse {
  summary: ServiceMetricsSummary
  items: ServiceMetricsItem[]
}

// Mock数据 - 服务指标数据
export const mockServiceMetrics: Record<string, ServiceMetricsResponse> = {
  "s3": {
    summary: {
      metrics: [
        { name: "latency", value: 15 },
        { name: "traffic", value: 1200 },
        { name: "errorRatio", value: 2 },
        { name: "saturation", value: 65 }
      ]
    },
    items: [
      {
        version: "v1.0.0",
        metrics: [
          { name: "latency", value: 13 },
          { name: "traffic", value: 800 },
          { name: "errorRatio", value: 1 },
          { name: "saturation", value: 55 }
        ]
      },
      {
        version: "v1.0.1",
        metrics: [
          { name: "latency", value: 18 },
          { name: "traffic", value: 300 },
          { name: "errorRatio", value: 3 },
          { name: "saturation", value: 70 }
        ]
      },
      {
        version: "v1.0.3",
        metrics: [
          { name: "latency", value: 20 },
          { name: "traffic", value: 100 },
          { name: "errorRatio", value: 5 },
          { name: "saturation", value: 80 }
        ]
      }
    ]
  },
  "stg": {
    summary: {
      metrics: [
        { name: "latency", value: 8 },
        { name: "traffic", value: 2000 },
        { name: "errorRatio", value: 0.5 },
        { name: "saturation", value: 45 }
      ]
    },
    items: [
      {
        version: "v1.0.0",
        metrics: [
          { name: "latency", value: 7 },
          { name: "traffic", value: 1400 },
          { name: "errorRatio", value: 0.3 },
          { name: "saturation", value: 40 }
        ]
      },
      {
        version: "v1.0.2",
        metrics: [
          { name: "latency", value: 10 },
          { name: "traffic", value: 600 },
          { name: "errorRatio", value: 0.8 },
          { name: "saturation", value: 55 }
        ]
      }
    ]
  },
  "meta": {
    summary: {
      metrics: [
        { name: "latency", value: 5 },
        { name: "traffic", value: 5000 },
        { name: "errorRatio", value: 0.1 },
        { name: "saturation", value: 30 }
      ]
    },
    items: [
      {
        version: "v1.0.3",
        metrics: [
          { name: "latency", value: 5 },
          { name: "traffic", value: 5000 },
          { name: "errorRatio", value: 0.1 },
          { name: "saturation", value: 30 }
        ]
      }
    ]
  },
  "mq": {
    summary: {
      metrics: [
        { name: "latency", value: 3 },
        { name: "traffic", value: 8000 },
        { name: "errorRatio", value: 0.05 },
        { name: "saturation", value: 25 }
      ]
    },
    items: [
      {
        version: "v1.0.1",
        metrics: [
          { name: "latency", value: 3 },
          { name: "traffic", value: 8000 },
          { name: "errorRatio", value: 0.05 },
          { name: "saturation", value: 25 }
        ]
      }
    ]
  },
  "worker": {
    summary: {
      metrics: [
        { name: "latency", value: 25 },
        { name: "traffic", value: 500 },
        { name: "errorRatio", value: 1.5 },
        { name: "saturation", value: 75 }
      ]
    },
    items: [
      {
        version: "v1.0.1",
        metrics: [
          { name: "latency", value: 25 },
          { name: "traffic", value: 500 },
          { name: "errorRatio", value: 1.5 },
          { name: "saturation", value: 75 }
        ]
      }
    ]
  },
  "mongodb": {
    summary: {
      metrics: [
        { name: "latency", value: 50 },
        { name: "traffic", value: 200 },
        { name: "errorRatio", value: 15 },
        { name: "saturation", value: 90 }
      ]
    },
    items: [
      {
        version: "v1.0.1",
        metrics: [
          { name: "latency", value: 50 },
          { name: "traffic", value: 200 },
          { name: "errorRatio", value: 15 },
          { name: "saturation", value: 90 }
        ]
      }
    ]
  }
}

// ==================== 告警记录相关类型定义 ====================

// 告警标签
export interface AlertLabel {
  key: string
  value: string
}

// 告警条目
export interface AlertIssue {
  id: string
  state: 'Open' | 'Closed'
  level: 'P0' | 'P1' | 'P2' | 'Warning'
  alertState: 'Pending' | 'InProcessing' | 'Restored' | 'AutoRestored'
  title: string
  labels: AlertLabel[]
  alertSince: string
  resolvedAt: string
}

// 告警列表响应
export interface AlertsResponse {
  items: AlertIssue[]
  next: string
}

// 告警评论
export interface AlertComment {
  createAt: string
  content: string
}

// 告警详情（包含AI分析记录）
export interface AlertDetail {
  id: string
  state: 'Open' | 'Closed'
  level: 'P0' | 'P1' | 'P2' | 'Warning'
  alertState: 'Restored' | 'AutoRestored' | 'InProcessing'
  title: string
  labels: AlertLabel[]
  alertSince: string
  comments: AlertComment[]
}

// ==================== 告警记录Mock数据 ====================

export const mockAlertsData: AlertsResponse = {
  items: [
    {
      id: 'alert-1',
      state: 'Closed',
      level: 'P1',
      alertState: 'Restored',
      title: 'yzh S3APIV2 s3apiv2.putobject 0_64K上传响应时间95值: 500.12ms > 450ms',
      labels: [
        { key: 'api', value: 's3apiv2.putobject' },
        { key: 'idc', value: 'yzh' },
        { key: 'org', value: 'kodo' },
        { key: 'prophet_service', value: 's3apiv2' },
        { key: 'prophet_type', value: 'app' }
      ],
      alertSince: '2025-09-01T19:14:12.382331146Z',
      resolvedAt: '2025-09-01T19:25:00.000Z'
    },
    {
      id: 'alert-2',
      state: 'Open',
      level: 'P0',
      alertState: 'InProcessing',
      title: 'bj S3APIV2 s3apiv2.getobject 下载响应时间95值: 1200.5ms > 1000ms',
      labels: [
        { key: 'api', value: 's3apiv2.getobject' },
        { key: 'idc', value: 'bj' },
        { key: 'org', value: 'kodo' },
        { key: 'prophet_service', value: 's3apiv2' },
        { key: 'prophet_type', value: 'app' }
      ],
      alertSince: '2025-09-01T20:10:15.123456789Z',
      resolvedAt: ''
    },
    {
      id: 'alert-3',
      state: 'Closed',
      level: 'P2',
      alertState: 'AutoRestored',
      title: 'sh MQ 消息队列积压数量: 15000 > 10000',
      labels: [
        { key: 'service', value: 'mq' },
        { key: 'idc', value: 'sh' },
        { key: 'org', value: 'kodo' },
        { key: 'prophet_service', value: 'mq' },
        { key: 'prophet_type', value: 'infra' }
      ],
      alertSince: '2025-09-01T18:30:00.000000000Z',
      resolvedAt: '2025-09-01T18:45:00.000Z'
    },
    {
      id: 'alert-4',
      state: 'Closed',
      level: 'Warning',
      alertState: 'Restored',
      title: 'gz Meta 数据库连接池使用率: 85% > 80%',
      labels: [
        { key: 'service', value: 'meta' },
        { key: 'idc', value: 'gz' },
        { key: 'org', value: 'kodo' },
        { key: 'prophet_service', value: 'meta' },
        { key: 'prophet_type', value: 'app' }
      ],
      alertSince: '2025-09-01T21:00:00.000000000Z',
      resolvedAt: ''
    },
    {
      id: 'alert-5',
      state: 'Closed',
      level: 'P1',
      alertState: 'Restored',
      title: 'sz STG 服务实例健康检查失败率: 15% > 10%',
      labels: [
        { key: 'service', value: 'stg' },
        { key: 'idc', value: 'sz' },
        { key: 'org', value: 'kodo' },
        { key: 'prophet_service', value: 'stg' },
        { key: 'prophet_type', value: 'app' }
      ],
      alertSince: '2025-09-01T17:20:00.000000000Z',
      resolvedAt: '2025-09-01T17:35:00.000Z'
    }
  ],
  next: '2025-09-01T16:00:00.000Z'
}

// ==================== 告警详情Mock数据 ====================

export const mockAlertDetails: Record<string, AlertDetail> = {
  'alert-1': {
    id: 'alert-1',
    state: 'Closed',
    level: 'P1',
    alertState: 'Restored',
    title: 'yzh S3APIV2 s3apiv2.putobject 0_64K上传响应时间95值: 500.12ms > 450ms',
    labels: [
      { key: 'api', value: 's3apiv2.putobject' },
      { key: 'idc', value: 'yzh' },
      { key: 'org', value: 'kodo' },
      { key: 'prophet_service', value: 's3apiv2' },
      { key: 'prophet_type', value: 'app' }
    ],
    alertSince: '2025-09-01T19:14:12.382331146Z',
    comments: [
      {
        createAt: '2025-09-01T19:15:00Z',
        content: `## AI分析结果

**问题类型**: 发版本导致的问题
**根因分析**: 新版本中的数据库查询优化存在问题，导致某些复杂查询性能下降

**处理建议**: 
- 立即回滚到稳定版本
- 优化数据库索引配置
- 增加监控告警阈值

**执行状态**: 已执行回滚操作，等待指标恢复正常

## 详细分析过程

### 1. 异常指标下钻分析
- **IDC维度**: 北京IDC延迟异常，P95延迟从120ms激增至500ms
- **API类型**: 主要影响s3apiv2.putobject接口，其他接口性能正常
- **请求大小**: 0-64KB请求延迟异常，大文件请求正常

### 2. 资源使用情况分析
- **CPU使用率**: 上升至85%，存在CPU瓶颈
- **内存使用率**: 正常(65%)
- **网络带宽**: 正常(45%)

### 3. 问题根因诊断
- **根本原因**: 北京IDC服务器CPU资源不足
- **影响范围**: 主要影响0-64KB文件上传功能
- **严重程度**: P1级别，影响用户体验

### 4. 解决方案执行
1. 立即扩容北京IDC服务器资源
2. 优化s3apiv2.putobject接口性能
3. 考虑流量调度到其他IDC
4. 加强资源监控和告警`
      }
    ]
  },
  'alert-2': {
    id: 'alert-2',
    state: 'Open',
    level: 'P0',
    alertState: 'InProcessing',
    title: 'bj S3APIV2 s3apiv2.getobject 下载响应时间95值: 1200.5ms > 1000ms',
    labels: [
      { key: 'api', value: 's3apiv2.getobject' },
      { key: 'idc', value: 'bj' },
      { key: 'org', value: 'kodo' },
      { key: 'prophet_service', value: 's3apiv2' },
      { key: 'prophet_type', value: 'app' }
    ],
    alertSince: '2025-09-01T20:10:15.123456789Z',
    comments: [
      {
        createAt: '2025-09-01T20:11:00Z',
        content: `## AI分析结果

**问题类型**: 严重性能问题
**根因分析**: 数据库连接池配置不当，导致连接等待时间过长

**处理建议**: 
- 立即调整数据库连接池配置
- 增加连接池大小
- 优化连接超时设置

**执行状态**: 正在处理中，等待配置生效

## 紧急处理方案

### 1. 性能指标分析
- **P95延迟**: 从200ms激增至1200ms
- **P99延迟**: 达到2000ms
- **错误率**: 从0.1%增加到2.5%
- **吞吐量**: 从1000 QPS下降到600 QPS

### 2. 资源瓶颈诊断
- **数据库连接池**: 使用率100%，连接池耗尽
- **CPU使用率**: 正常(45%)
- **内存使用率**: 正常(60%)

### 3. 紧急处理措施
1. 立即增加数据库连接池大小从20到50
2. 调整连接超时时间从30s到10s
3. 启用连接池监控
4. 考虑读写分离缓解压力`
      }
    ]
  },
  'alert-3': {
    id: 'alert-3',
    state: 'Closed',
    level: 'P2',
    alertState: 'AutoRestored',
    title: 'sh MQ 消息队列积压数量: 15000 > 10000',
    labels: [
      { key: 'service', value: 'mq' },
      { key: 'idc', value: 'sh' },
      { key: 'org', value: 'kodo' },
      { key: 'prophet_service', value: 'mq' },
      { key: 'prophet_type', value: 'infra' }
    ],
    alertSince: '2025-09-01T18:30:00.000000000Z',
    comments: [
      {
        createAt: '2025-09-01T18:35:00Z',
        content: `## AI分析结果

**问题类型**: 消息队列积压
**根因分析**: 消费者服务重启导致消息积压

**处理建议**: 
- 增加消费者实例数量
- 优化消息处理逻辑
- 监控队列积压情况

**执行状态**: 已自动恢复，队列积压已清理

## 问题分析过程

### 1. 队列积压分析
- **积压数量**: 15000条消息，超过阈值10000条
- **积压率**: 50%
- **增长趋势**: 18:25开始快速增长
- **消息类型**: 主要是订单处理消息，占80%

### 2. 消费者状态分析
- **服务状态**: 订单处理服务在18:25重启
- **消费速率**: 从1000 msg/s下降到0，重启后恢复正常
- **错误日志**: 发现服务重启期间的连接错误

### 3. 自动恢复过程
1. 服务自动重启恢复
2. 消费速率恢复正常
3. 队列积压逐步清理
4. 系统状态恢复正常`
      }
    ]
  },
  'alert-4': {
    id: 'alert-4',
    state: 'Closed',
    level: 'Warning',
    alertState: 'Restored',
    title: 'gz Meta 数据库连接池使用率: 85% > 80%',
    labels: [
      { key: 'service', value: 'meta' },
      { key: 'idc', value: 'gz' },
      { key: 'org', value: 'kodo' },
      { key: 'prophet_service', value: 'meta' },
      { key: 'prophet_type', value: 'app' }
    ],
    alertSince: '2025-09-01T21:00:00.000000000Z',
    comments: [
      {
        createAt: '2025-09-01T21:01:00Z',
        content: `## AI分析结果

**问题类型**: 资源使用率告警
**根因分析**: 数据库连接池使用率偏高，需要关注

**处理建议**: 
- 监控连接池使用情况
- 考虑增加连接池大小
- 优化数据库查询

**执行状态**: 正在监控中

## 连接池使用分析

### 1. 连接池状态
- **当前连接数**: 85个活跃连接
- **最大连接数**: 100个
- **使用率**: 85%
- **平均连接时长**: 5分钟
- **最长连接时长**: 30分钟

### 2. 数据库性能分析
- **查询性能**: 部分复杂查询执行时间较长，平均2秒
- **锁等待**: 存在少量锁等待，平均等待时间100ms
- **事务分析**: 存在部分长事务，平均事务时长10秒

### 3. 优化建议
1. 优化慢查询，添加索引
2. 减少长事务，拆分复杂操作
3. 考虑增加连接池大小到120
4. 设置连接超时时间`
      }
    ]
  },
  'alert-5': {
    id: 'alert-5',
    state: 'Closed',
    level: 'P1',
    alertState: 'Restored',
    title: 'sz STG 服务实例健康检查失败率: 15% > 10%',
    labels: [
      { key: 'service', value: 'stg' },
      { key: 'idc', value: 'sz' },
      { key: 'org', value: 'kodo' },
      { key: 'prophet_service', value: 'stg' },
      { key: 'prophet_type', value: 'app' }
    ],
    alertSince: '2025-09-01T17:20:00.000000000Z',
    comments: [
      {
        createAt: '2025-09-01T17:25:00Z',
        content: `## AI分析结果

**问题类型**: 服务健康检查失败
**根因分析**: 服务实例内存使用率过高导致健康检查失败

**处理建议**: 
- 重启异常实例
- 优化内存使用
- 调整健康检查阈值

**执行状态**: 已处理完成，服务恢复正常

## 健康检查失败分析

### 1. 失败情况统计
- **失败率**: 15%，超过阈值10%
- **影响实例**: 3个实例（stg-03、stg-07、stg-12）
- **失败类型**: 主要是超时失败，占80%

### 2. 实例状态分析
- **内存使用率**: 失败实例达到95%
- **CPU使用率**: 正常
- **服务状态**: 进程存在但响应缓慢
- **错误日志**: 内存不足警告，GC频繁

### 3. 处理措施
1. 重启内存使用率过高的实例
2. 调整JVM堆内存大小
3. 优化内存使用
4. 加强资源监控`
      }
    ]
  }
}

// Mock数据 - 告警规则变更记录
export const mockAlertRuleChangelog: AlertRuleChangelogResponse = {
  items: [
    {
      name: "p98_latency_too_high",
      editTime: "2024-01-04T12:00:00Z",
      scope: "service:stg",
      values: [
        {
          name: "threshold",
          old: "10",
          new: "15"
        },
        {
          name: "watchTimeDuration",
          old: "3min",
          new: "5min"
        }
      ],
      reason: "由于业务增长，系统负载增加，原有10ms的延时阈值过于严格，导致频繁告警。经过AI分析历史数据，建议将阈值调整为15ms，既能及时发现性能问题，又避免误报。"
    },
    {
      name: "saturation_too_high",
      editTime: "2024-01-03T15:00:00Z",
      scope: "service:stg",
      values: [
        {
          name: "threshold",
          old: "50",
          new: "45"
        }
      ],
      reason: "监控发现系统在50%饱和度时已出现性能下降，提前预警有助于避免系统过载。调整后可以更早发现资源瓶颈，确保服务稳定性。"
    },
    {
      name: "p98_latency_too_high",
      editTime: "2024-01-03T10:00:00Z",
      scope: "service:mongo",
      values: [
        {
          name: "threshold",
          old: "10",
          new: "5"
        }
      ],
      reason: "MongoDB服务经过优化后性能显著提升，原有10ms阈值已不适用。调整为5ms可以更精确地监控数据库性能，及时发现潜在问题。"
    },
    {
      name: "error_rate_too_high",
      editTime: "2024-01-01T15:00:00Z",
      scope: "service:meta",
      values: [
        {
          name: "threshold",
          old: "10",
          new: "5"
        }
      ],
      reason: "Meta服务作为核心服务，对错误率要求更加严格。将错误告警阈值从10降低到5，可以更敏感地发现服务异常，确保数据一致性。"
    }
  ],
}

// ==================== 数据持久化函数 ====================
// 使用 localStorage 实现前端数据持久化

/**
 * 保存数据到 localStorage
 */
const saveDataToStorage = () => {
  try {
    localStorage.setItem('mockServiceActiveVersions', JSON.stringify(mockServiceActiveVersions))
    localStorage.setItem('mockAvailableVersions', JSON.stringify(mockAvailableVersions))
    console.log('数据已保存到 localStorage')
  } catch (error) {
    console.error('保存数据到 localStorage 失败:', error)
  }
}

/**
 * 从 localStorage 加载数据
 */
const loadDataFromStorage = () => {
  try {
    const activeVersionsData = localStorage.getItem('mockServiceActiveVersions')
    const availableVersionsData = localStorage.getItem('mockAvailableVersions')
    
    if (activeVersionsData) {
      const parsedData = JSON.parse(activeVersionsData)
      Object.assign(mockServiceActiveVersions, parsedData)
      console.log('已从 localStorage 加载活跃版本数据')
    }
    
    if (availableVersionsData) {
      const parsedData = JSON.parse(availableVersionsData)
      Object.assign(mockAvailableVersions, parsedData)
      console.log('已从 localStorage 加载可发布版本数据')
    }
  } catch (error) {
    console.error('从 localStorage 加载数据失败:', error)
  }
}

// 页面加载时自动从 localStorage 恢复数据
loadDataFromStorage()

// ==================== 服务告警状态管理 ====================
// 管理服务节点的告警状态，用于改变拓扑图中节点的颜色

/**
 * 服务告警状态类型
 */
export type ServiceAlertStatus = 'normal' | 'pending' | 'processing'

/**
 * 服务告警状态映射
 */
export const serviceAlertStatusMap: Record<string, ServiceAlertStatus> = {}

/**
 * 保存服务告警状态到 localStorage
 */
const saveServiceAlertStatus = () => {
  try {
    localStorage.setItem('serviceAlertStatusMap', JSON.stringify(serviceAlertStatusMap))
    console.log('服务告警状态已保存到 localStorage')
  } catch (error) {
    console.error('保存服务告警状态失败:', error)
  }
}

/**
 * 从 localStorage 加载服务告警状态
 */
const loadServiceAlertStatus = () => {
  try {
    const data = localStorage.getItem('serviceAlertStatusMap')
    if (data) {
      const parsedData = JSON.parse(data)
      Object.assign(serviceAlertStatusMap, parsedData)
      console.log('已从 localStorage 加载服务告警状态')
    }
  } catch (error) {
    console.error('从 localStorage 加载服务告警状态失败:', error)
  }
}

/**
 * 根据告警状态更新服务状态
 * @param serviceName 服务名称
 * @param alertState 告警状态
 */
export const updateServiceAlertStatus = (serviceName: string, alertState: string) => {
  let status: ServiceAlertStatus = 'normal'
  
  switch (alertState) {
    case 'Pending':
      status = 'pending'
      break
    case 'InProcessing':
      status = 'processing'
      break
    case 'Restored':
    case 'AutoRestored':
    default:
      status = 'normal'
      break
  }
  
  serviceAlertStatusMap[serviceName] = status
  saveServiceAlertStatus()
  
  console.log(`服务 ${serviceName} 告警状态更新为: ${status} (${alertState})`)
  return status
}

/**
 * 获取服务告警状态
 * @param serviceName 服务名称
 */
export const getServiceAlertStatus = (serviceName: string): ServiceAlertStatus => {
  return serviceAlertStatusMap[serviceName] || 'normal'
}

/**
 * 清除服务告警状态
 * @param serviceName 服务名称
 */
export const clearServiceAlertStatus = (serviceName: string) => {
  delete serviceAlertStatusMap[serviceName]
  saveServiceAlertStatus()
  console.log(`已清除服务 ${serviceName} 的告警状态`)
}

// 页面加载时自动从 localStorage 恢复服务告警状态
loadServiceAlertStatus()

// ==================== 服务版本告警状态管理 ====================
// 让首页服务详情中的各版本颜色与告警状态同步

/**
 * 服务版本告警状态映射：service -> version -> status
 */
export const serviceVersionAlertStatusMap: Record<string, Record<string, ServiceAlertStatus>> = {}

const saveServiceVersionAlertStatus = () => {}

const loadServiceVersionAlertStatus = () => {}

/**
 * 根据告警状态更新服务版本状态
 * Pending -> pending, InProcessing -> processing, Restored/AutoRestored -> normal
 */
export const updateServiceVersionAlertStatus = (serviceName: string, version: string, alertState: string) => {
  if (!serviceVersionAlertStatusMap[serviceName]) {
    serviceVersionAlertStatusMap[serviceName] = {}
  }

  let status: ServiceAlertStatus = 'normal'
  switch (alertState) {
    case 'Pending':
      status = 'pending'
      break
    case 'InProcessing':
      status = 'processing'
      break
    case 'Restored':
    case 'AutoRestored':
    default:
      status = 'normal'
      break
  }

  serviceVersionAlertStatusMap[serviceName][version] = status
  saveServiceVersionAlertStatus()
  console.log(`服务 ${serviceName} 版本 ${version} 告警状态更新为: ${status} (${alertState})`)
  return status
}

/**
 * 获取服务版本告警状态
 */
export const getServiceVersionAlertStatus = (serviceName: string, version: string): ServiceAlertStatus => {
  return serviceVersionAlertStatusMap[serviceName]?.[version] || 'normal'
}

/**
 * 清除服务版本告警状态
 */
export const clearServiceVersionAlertStatus = (serviceName: string, version?: string) => {
  if (version) {
    if (serviceVersionAlertStatusMap[serviceName]) {
      delete serviceVersionAlertStatusMap[serviceName][version]
    }
  } else {
    delete serviceVersionAlertStatusMap[serviceName]
  }
  saveServiceVersionAlertStatus()
  console.log(`已清除服务 ${serviceName} ${version ? '版本 ' + version : '所有版本'} 的告警状态`)
}

// 页面加载时不再从 localStorage 恢复服务版本告警状态（禁用持久化）

// ==================== 发布任务状态管理 ====================
// 管理服务的发布任务状态，用于显示发布指示器

/**
 * 发布任务状态类型
 */
export type DeploymentStatus = 'idle' | 'deploying'

/**
 * 服务发布状态映射
 */
export const serviceDeploymentStatusMap: Record<string, DeploymentStatus> = {}

/**
 * 保存服务发布状态到 localStorage
 */
const saveServiceDeploymentStatus = () => {
  try {
    localStorage.setItem('serviceDeploymentStatusMap', JSON.stringify(serviceDeploymentStatusMap))
    console.log('服务发布状态已保存到 localStorage')
  } catch (error) {
    console.error('保存服务发布状态失败:', error)
  }
}

/**
 * 从 localStorage 加载服务发布状态
 */
const loadServiceDeploymentStatus = () => {
  try {
    const data = localStorage.getItem('serviceDeploymentStatusMap')
    if (data) {
      const parsedData = JSON.parse(data)
      Object.assign(serviceDeploymentStatusMap, parsedData)
      console.log('已从 localStorage 加载服务发布状态')
    }
  } catch (error) {
    console.error('从 localStorage 加载服务发布状态失败:', error)
  }
}

/**
 * 设置服务发布状态
 * @param serviceName 服务名称
 * @param status 发布状态
 */
export const setServiceDeploymentStatus = (serviceName: string, status: DeploymentStatus) => {
  serviceDeploymentStatusMap[serviceName] = status
  saveServiceDeploymentStatus()
  console.log(`服务 ${serviceName} 发布状态更新为: ${status}`)
}

/**
 * 获取服务发布状态
 * @param serviceName 服务名称
 */
export const getServiceDeploymentStatus = (serviceName: string): DeploymentStatus => {
  return serviceDeploymentStatusMap[serviceName] || 'idle'
}

/**
 * 清除服务发布状态
 * @param serviceName 服务名称
 */
export const clearServiceDeploymentStatus = (serviceName: string) => {
  delete serviceDeploymentStatusMap[serviceName]
  saveServiceDeploymentStatus()
  console.log(`已清除服务 ${serviceName} 的发布状态`)
}

// 页面加载时自动从 localStorage 恢复服务发布状态
loadServiceDeploymentStatus()

// ==================== 数据操作函数 ====================
// 这些函数模拟后端操作数据的过程，用于支持前端的发布和回滚功能

/**
 * 从可发布版本列表中移除指定版本
 * @param serviceName 服务名称
 * @param version 要移除的版本
 */
export const removeVersionFromAvailable = (serviceName: string, version: string) => {
  if (mockAvailableVersions[serviceName]) {
    const index = mockAvailableVersions[serviceName].items.findIndex(item => item.version === version)
    if (index >= 0) {
      mockAvailableVersions[serviceName].items.splice(index, 1)
      console.log(`已从 ${serviceName} 的可发布版本列表中移除 ${version}`)
      saveDataToStorage() // 保存到 localStorage
    }
  }
}

/**
 * 将版本添加到可发布版本列表中
 * @param serviceName 服务名称
 * @param version 要添加的版本
 */
export const addVersionToAvailable = (serviceName: string, version: string) => {
  if (mockAvailableVersions[serviceName]) {
    // 检查是否已存在
    const existing = mockAvailableVersions[serviceName].items.find(item => item.version === version)
    if (!existing) {
      mockAvailableVersions[serviceName].items.push({
        version: version,
        createTime: new Date().toISOString()
      })
      console.log(`已将 ${version} 添加到 ${serviceName} 的可发布版本列表中`)
      saveDataToStorage() // 保存到 localStorage
    }
  }
}

/**
 * 从服务活跃版本中移除指定版本
 * @param serviceName 服务名称
 * @param version 要移除的版本
 */
export const removeVersionFromServiceActiveVersions = (serviceName: string, version: string) => {
  if (mockServiceActiveVersions[serviceName]) {
    const index = mockServiceActiveVersions[serviceName].items.findIndex(v => v.version === version)
    if (index >= 0) {
      mockServiceActiveVersions[serviceName].items.splice(index, 1)
      console.log(`已从 ${serviceName} 的服务活跃版本中移除 ${version}`)
      saveDataToStorage() // 保存到 localStorage
    }
  }
}

/**
 * 将版本添加到服务活跃版本中（用于饼状图显示）
 * @param serviceName 服务名称
 * @param version 要添加的版本
 */
export const addVersionToServiceActiveVersions = (serviceName: string, version: string) => {
  if (mockServiceActiveVersions[serviceName]) {
    // 检查是否已存在
    const existing = mockServiceActiveVersions[serviceName].items.find(v => v.version === version)
    if (!existing) {
      const now = new Date()
      const endTime = new Date(now.getTime() + 2 * 60 * 60 * 1000) // 两小时后
      
      // 计算总实例数，用于确定15%对应的实例数
      const totalInstances = mockServiceActiveVersions[serviceName].items.reduce((sum, item) => sum + item.instances, 0)
      const newVersionInstances = Math.max(1, Math.round(totalInstances * 0.15)) // 新版本占15%，最少1个实例
      
      // 如果有现有版本，从最早发布的版本（第一个）减少15%
      if (mockServiceActiveVersions[serviceName].items.length > 0) {
        const earliestVersion = mockServiceActiveVersions[serviceName].items[0]
        const reduceInstances = Math.min(earliestVersion.instances, newVersionInstances)
        earliestVersion.instances = Math.max(0, earliestVersion.instances - reduceInstances)
        console.log(`从最早版本 ${earliestVersion.version} 减少 ${reduceInstances} 个实例`)
      }
      
      mockServiceActiveVersions[serviceName].items.push({
        version: version,
        deployID: `deploy-${Date.now()}`,
        startTime: now.toISOString(),
        estimatedCompletionTime: endTime.toISOString(),
        instances: newVersionInstances, // 新版本占15%
        health: "Normal"
      })
      console.log(`已将 ${version} 添加到 ${serviceName} 的服务活跃版本中，实例数: ${newVersionInstances}`)
      saveDataToStorage() // 保存到 localStorage
    }
  }
}

/**
 * 创建发布任务 - 模拟后端操作
 * @param serviceName 服务名称
 * @param version 要发布的版本
 */
export const createReleaseTask = (serviceName: string, version: string) => {
  console.log(`开始创建发布任务: ${serviceName} -> ${version}`)
  console.log('修改前的可发布版本列表:', mockAvailableVersions[serviceName]?.items)
  console.log('修改前的服务活跃版本列表:', mockServiceActiveVersions[serviceName]?.items)
  
  // 1. 从可发布版本列表中移除
  removeVersionFromAvailable(serviceName, version)
  
  // 2. 添加到服务活跃版本中
  addVersionToServiceActiveVersions(serviceName, version)
  
  // 3. 设置服务发布状态为发布中
  setServiceDeploymentStatus(serviceName, 'deploying')
  
  console.log('修改后的可发布版本列表:', mockAvailableVersions[serviceName]?.items)
  console.log('修改后的服务活跃版本列表:', mockServiceActiveVersions[serviceName]?.items)
  console.log(`发布任务创建成功: ${serviceName} -> ${version}`)
}

/**
 * 回滚版本 - 模拟后端操作
 * @param serviceName 服务名称
 * @param version 要回滚的版本
 */
export const rollbackVersion = (serviceName: string, version: string) => {
  console.log(`开始回滚版本: ${serviceName} -> ${version}`)
  console.log('回滚前的服务活跃版本列表:', mockServiceActiveVersions[serviceName]?.items)
  console.log('回滚前的可发布版本列表:', mockAvailableVersions[serviceName]?.items)
  
  // 1. 从服务活跃版本中移除
  removeVersionFromServiceActiveVersions(serviceName, version)
  
  // 2. 恢复到可发布版本列表中
  addVersionToAvailable(serviceName, version)
  
  // 3. 清除服务发布状态
  clearServiceDeploymentStatus(serviceName)
  
  console.log('回滚后的服务活跃版本列表:', mockServiceActiveVersions[serviceName]?.items)
  console.log('回滚后的可发布版本列表:', mockAvailableVersions[serviceName]?.items)
  console.log(`版本回滚成功: ${serviceName} -> ${version}`)
}
