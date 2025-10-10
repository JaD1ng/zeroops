<template>
  <div class="module-record">
    <div class="record-header">
      <div class="record-info">
        <div class="status-icon">
          <el-icon v-if="record.status === '告警'"><Warning /></el-icon>
          <el-icon v-else-if="record.status === '回滚'"><Refresh /></el-icon>
          <div v-else class="status-dot" :class="getStatusDotClass(record.status)"></div>
        </div>
        <div class="module-info">
          <span class="module-name">{{ record.module }}</span>
          <span class="action">- {{ record.action }}</span>
          <el-tag
            :type="getStatusType(record.status)"
            size="small"
            class="status-tag"
          >
            {{ record.status }}
          </el-tag>
        </div>
      </div>
    </div>

    <div class="record-details">
      <div class="timestamp">{{ record.timestamp }}</div>
      <div class="details">{{ record.details }}</div>
      
      <!-- AI 思考过程 -->
      <div v-if="record.thoughts" class="thoughts-section">
        <el-button
          type="text"
          size="small"
          @click="toggleThoughts"
          class="thoughts-btn"
        >
          <el-icon><Brain /></el-icon>
          Show Thoughts
          <el-icon>
            <ArrowUp v-if="showThoughts" />
            <ArrowDown v-else />
          </el-icon>
        </el-button>
        <div v-if="showThoughts" class="thoughts-content">
          {{ record.thoughts }}
        </div>
      </div>

      <!-- 事件详细数据 -->
      <div v-if="record.eventData" class="event-data">
        <div class="event-data-title">事件详细数据:</div>
        <pre class="event-data-content">{{ JSON.stringify(record.eventData, null, 2) }}</pre>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { ModuleRecord } from '@/stores/app'

interface Props {
  record: ModuleRecord
}

const props = defineProps<Props>()

const showThoughts = ref(false)

const toggleThoughts = () => {
  showThoughts.value = !showThoughts.value
}

const getStatusDotClass = (status: string) => {
  switch (status) {
    case '成功':
      return 'bg-emerald-500'
    case '失败':
      return 'bg-rose-500'
    case '告警':
      return 'bg-amber-500'
    case '回滚':
      return 'bg-blue-500'
    default:
      return 'bg-slate-400'
  }
}

const getStatusType = (status: string) => {
  switch (status) {
    case '成功':
      return 'success'
    case '失败':
      return 'danger'
    case '告警':
      return 'warning'
    case '回滚':
      return 'primary'
    default:
      return 'info'
  }
}
</script>

<style scoped>
.module-record {
  background: white;
  border-radius: 8px;
  padding: 12px;
  border: 1px solid #e2e8f0;
}

.record-header {
  margin-bottom: 8px;
}

.record-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.module-info {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
}

.module-name {
  font-weight: 500;
  font-size: 14px;
}

.action {
  font-size: 12px;
  color: #6b7280;
}

.status-tag {
  font-size: 12px;
}

.record-details {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.timestamp {
  font-size: 12px;
  color: #6b7280;
}

.details {
  font-size: 14px;
  color: #374151;
  word-break: break-word;
  overflow-wrap: break-word;
}

.thoughts-section {
  margin-top: 8px;
}

.thoughts-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  padding: 4px 8px;
  height: auto;
}

.thoughts-content {
  margin-top: 8px;
  padding: 8px;
  background: #f8fafc;
  border-radius: 4px;
  border-left: 2px solid #3b82f6;
  font-size: 12px;
  color: #4b5563;
  white-space: pre-wrap;
}

.event-data {
  margin-top: 8px;
}

.event-data-title {
  font-weight: 500;
  font-size: 12px;
  margin-bottom: 4px;
}

.event-data-content {
  padding: 8px;
  background: #f8fafc;
  border-radius: 4px;
  border-left: 2px solid #10b981;
  font-size: 11px;
  color: #4b5563;
  white-space: pre-wrap;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  overflow-x: auto;
  max-height: 300px;
  overflow-y: auto;
  word-break: break-all;
}
</style>
