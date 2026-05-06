<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onUnmounted } from 'vue'
import { useProxyStore } from '@/stores/proxy'
import type { LogLevel } from '@/types'

const proxyStore = useProxyStore()

const filterLevel = ref<LogLevel | 'all'>('all')
const searchQuery = ref('')
const autoScroll = ref(true)
const logContainer = ref<HTMLElement | null>(null)

onMounted(() => {
  setupLogListener()
})

onUnmounted(() => {
  window.removeEventListener('log:incoming', handleLogIncoming as any)
})

function setupLogListener() {
  setInterval(() => {
    if (autoScroll.value && logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  }, 500)
}

function handleLogIncoming() {
  if (autoScroll.value && logContainer.value) {
    nextTick(() => {
      logContainer.value!.scrollTop = logContainer.value!.scrollHeight
    })
  }
}

const filteredLogs = computed(() => {
  let logs = proxyStore.logs
  
  if (filterLevel.value !== 'all') {
    logs = logs.filter(log => log.level === filterLevel.value)
  }
  
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    logs = logs.filter(log => log.raw.toLowerCase().includes(query))
  }
  
  return logs
})

function clearLogs() {
  proxyStore.clearLogs()
}
</script>

<template>
  <div class="h-full flex flex-col p-6">
    <div class="max-w-6xl mx-auto w-full flex-1 flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-xl font-semibold">日志查看器</h2>
        <div class="flex items-center gap-3">
          <label class="flex items-center gap-2 text-sm text-text-muted">
            <input type="checkbox" v-model="autoScroll" class="rounded" />
            自动滚动
          </label>
          <button @click="clearLogs" class="btn-outline px-3 py-1 text-sm">
            清空
          </button>
        </div>
      </div>

      <div class="flex items-center gap-3 mb-4">
        <div class="flex-1 relative">
          <input
            v-model="searchQuery"
            placeholder="搜索日志..."
            class="input-field pl-10"
          />
          <svg class="w-5 h-5 text-text-muted absolute left-3 top-1/2 -translate-y-1/2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </div>

        <select v-model="filterLevel" class="input-field w-32">
          <option value="all">全部</option>
          <option value="info">INFO</option>
          <option value="warn">WARN</option>
          <option value="error">ERROR</option>
        </select>
      </div>

      <div class="flex-1 bg-bg-primary rounded-lg border border-border p-4 font-mono text-sm overflow-y-auto" ref="logContainer">
        <div v-if="filteredLogs.length === 0" class="text-center py-12 text-text-muted">
          {{ searchQuery ? '没有匹配的日志' : '暂无日志记录...' }}
        </div>
        <div
          v-for="(log, i) in filteredLogs"
          :key="i"
          class="log-line py-1 px-2 rounded hover:bg-bg-secondary transition-colors"
          :class="{
            'log-info': log.level === 'info',
            'log-warn': log.level === 'warn',
            'log-error': log.level === 'error',
          }"
        >
          <span class="text-text-muted mr-4">{{ log.timestamp }}</span>
          <span
            v-if="log.level !== 'info'"
            class="text-xs px-1.5 py-0.5 rounded mr-2"
            :class="{
              'bg-yellow-900/50 text-yellow-300': log.level === 'warn',
              'bg-red-900/50 text-red-300': log.level === 'error',
            }"
          >
            {{ log.level.toUpperCase() }}
          </span>
          <span>{{ log.message }}</span>
        </div>
      </div>

      <div class="flex items-center justify-between mt-3 text-sm text-text-muted">
        <span>{{ filteredLogs.length }} 条日志</span>
        <span>最大保留: {{ proxyStore.maxLogs }} 条</span>
      </div>
    </div>
  </div>
</template>
