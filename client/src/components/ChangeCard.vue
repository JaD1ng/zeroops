<template>
  <el-card class="change-card" shadow="hover" :class="{ 'expandable': item.batches }" @click="handleCardClick">
    <template #header>
      <div class="card-header">
        <div class="card-title">
          <div class="status-dot" :class="getStatusDotClass(item.state)"></div>
          <span class="service-name">{{ item.service }} 服务 {{ item.version }} 发布</span>
          <span class="status-text">
            <span v-if="item.state === '发布中'">当前状态：完成{{ item.progress || 0 }}%</span>
            <span v-else-if="item.state === '灰度中'">当前状态：{{ item.progress || 0 }}% 灰度中，当前{{ item.ok ? '正常' : '异常' }}</span>
            <span v-else-if="item.state === '已完成'">发布完成，{{ item.ok ? '正常' : '异常' }}</span>
          </span>
        </div>
        <div v-if="item.batches" class="expand-indicator">
          <el-icon>
            <ArrowUp v-if="isExpanded" />
            <ArrowDown v-else />
          </el-icon>
        </div>
      </div>
    </template>

    <div class="card-content">
      <!-- 进度条 -->
      <div v-if="typeof item.progress === 'number'" class="progress-section">
        <el-progress :percentage="item.progress" :color="getProgressColor(item.state)" />
      </div>

      <!-- 批次详情 -->
      <div v-if="item.batches && isExpanded" class="batches-section">
        <BatchDetail
          v-for="(batch, idx) in item.batches"
          :key="idx"
          :batch="batch"
        />
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { ChangeItem } from '@/stores/app'
import BatchDetail from './BatchDetail.vue'

interface Props {
  item: ChangeItem
}

const props = defineProps<Props>()

const isExpanded = ref(false)

const toggleExpanded = () => {
  isExpanded.value = !isExpanded.value
}

const handleCardClick = () => {
  if (props.item.batches) {
    toggleExpanded()
  }
}

const getStatusDotClass = (state: string) => {
  switch (state) {
    case '已完成':
      return 'bg-emerald-500'
    case '灰度中':
      return 'bg-emerald-500'
    case '发布中':
      return 'bg-orange-500'
    default:
      return 'bg-slate-400'
  }
}

const getProgressColor = (state: string) => {
  switch (state) {
    case '已完成':
      return '#10b981'
    case '灰度中':
      return '#10b981'
    case '发布中':
      return '#f59e0b'
    default:
      return '#6b7280'
  }
}
</script>

<style scoped>
.change-card {
  margin-bottom: 16px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
  overflow: hidden;
}

.status-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  flex-shrink: 0;
}

.service-name {
  font-weight: 600;
  font-size: 16px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.status-text {
  font-size: 14px;
  color: #6b7280;
  margin-left: 8px;
  white-space: nowrap;
}

.expand-indicator {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #6b7280;
  flex-shrink: 0;
}

.expandable {
  cursor: pointer;
  transition: all 0.2s ease;
}

.expandable:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.card-content {
  padding-top: 16px;
}

.progress-section {
  margin-bottom: 16px;
}

.batches-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
</style>
