<template>
  <el-card class="alarm-change-card" shadow="hover" :class="{ 'expandable': item.details }" @click="handleCardClick">
    <div class="card-content">
      <div class="card-header">
        <div class="alarm-info">
          <div class="service-name">{{ item.service }}</div>
          <div class="change-description">{{ item.change }}</div>
          <div class="timestamp">{{ item.timestamp }}</div>
        </div>
        <div v-if="item.details" class="expand-indicator">
          <el-icon>
            <ArrowUp v-if="isExpanded" />
            <ArrowDown v-else />
          </el-icon>
        </div>
      </div>

      <!-- 详细信息 -->
      <div v-if="isExpanded && item.details" class="details-section">
        <div class="details-content">
          {{ item.details }}
        </div>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { AlarmChangeItem } from '@/stores/app'

interface Props {
  item: AlarmChangeItem
}

const props = defineProps<Props>()

const isExpanded = ref(false)

const toggleExpanded = () => {
  isExpanded.value = !isExpanded.value
}

const handleCardClick = () => {
  if (props.item.details) {
    toggleExpanded()
  }
}
</script>

<style scoped>
.alarm-change-card {
  margin-bottom: 16px;
}

.card-content {
  padding: 16px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
}

.alarm-info {
  flex: 1;
  min-width: 0;
  overflow: hidden;
}

.service-name {
  font-weight: 500;
  font-size: 14px;
  margin-bottom: 4px;
}

.change-description {
  font-size: 14px;
  color: #374151;
  margin-bottom: 4px;
  word-break: break-word;
  overflow-wrap: break-word;
  max-width: 100%;
}

.timestamp {
  font-size: 12px;
  color: #6b7280;
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

.details-section {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid #e5e7eb;
}

.details-content {
  padding: 12px;
  background: #f8fafc;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
  font-size: 14px;
  color: #4b5563;
  line-height: 1.6;
  word-break: break-word;
  overflow-wrap: break-word;
  max-height: 400px;
  overflow-y: auto;
}
</style>
