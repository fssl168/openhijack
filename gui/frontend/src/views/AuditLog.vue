<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { AuditService } from '@/services'
import { useUIStore } from '@/stores/ui'
import type { AuditEntry } from '@/types'

const uiStore = useUIStore()

const entries = ref<AuditEntry[]>([])
const loading = ref(false)
const logPath = ref('')
const offset = ref(0)
const hasMore = ref(false)
const PAGE_SIZE = 100

const statusFilter = ref<'all' | '2xx' | '4xx' | '5xx'>('all')
const pathFilter = ref('')

const filteredEntries = computed(() => {
  let list = entries.value
  if (statusFilter.value !== 'all') {
    list = list.filter((e) => {
      const cls = Math.floor(e.status / 100)
      if (statusFilter.value === '2xx') return cls === 2
      if (statusFilter.value === '4xx') return cls === 4
      if (statusFilter.value === '5xx') return cls === 5
      return true
    })
  }
  if (pathFilter.value) {
    const q = pathFilter.value.toLowerCase()
    list = list.filter((e) => e.path?.toLowerCase().includes(q))
  }
  return list
})

onMounted(() => {
  loadLogs()
  loadLogPath()
})

async function loadLogs(reset = true) {
  loading.value = true
  try {
    if (reset) {
      offset.value = 0
      entries.value = []
    }
    const batch = await AuditService.getLogs(PAGE_SIZE, offset.value)
    if (batch.length > 0) {
      entries.value = reset ? batch : [...entries.value, ...batch]
      hasMore.value = batch.length === PAGE_SIZE
      offset.value += batch.length
    } else {
      hasMore.value = false
    }
  } catch (e: any) {
    uiStore.showNotification(`加载审计日志失败: ${e?.message || '未知错误'}`, 'error', 5000)
  } finally {
    loading.value = false
  }
}

async function loadLogPath() {
  logPath.value = await AuditService.getLogPath()
}

async function clearLogs() {
  if (!confirm('确定要清空所有审计日志吗？此操作不可恢复。')) return
  const result = await AuditService.clearLogs()
  if (result.success) {
    entries.value = []
    offset.value = 0
    hasMore.value = false
    uiStore.showNotification('审计日志已清空', 'success', 3000)
  } else {
    uiStore.showNotification(result.error || '清空失败', 'error', 5000)
  }
}

function statusClass(status: number): string {
  const cls = Math.floor(status / 100)
  if (cls === 2) return 'text-green-400'
  if (cls === 4) return 'text-yellow-400'
  if (cls === 5) return 'text-red-400'
  return 'text-gray-400'
}

function formatTime(ts: string): string {
  if (!ts) return ''
  try {
    const d = new Date(ts)
    return d.toLocaleString()
  } catch {
    return ts
  }
}
</script>

<template>
  <div class="h-full flex flex-col p-4 md:p-6" role="tabpanel" aria-label="审计日志面板">
    <div class="max-w-6xl mx-auto w-full flex-1 flex flex-col">
      <header class="flex flex-col md:flex-row md:items-center md:justify-between gap-3 mb-4">
        <div>
          <h2 class="text-xl font-bold text-text-primary">📜 审计日志</h2>
          <p class="text-xs text-text-secondary mt-1" v-if="logPath">
            日志文件: {{ logPath }}
          </p>
        </div>
        <div class="flex items-center gap-2">
          <button
            @click="loadLogs(true)"
            :disabled="loading"
            class="px-3 py-1.5 bg-primary hover:bg-primary-light text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50"
          >
            🔄 刷新
          </button>
          <button
            @click="clearLogs"
            class="px-3 py-1.5 bg-red-700 hover:bg-red-600 text-white rounded-lg text-sm font-medium transition-colors"
          >
            🗑️ 清空
          </button>
        </div>
      </header>

      <section class="flex flex-col sm:flex-row gap-2 mb-3" aria-label="筛选器">
        <select
          v-model="statusFilter"
          class="px-3 py-1.5 bg-bg-secondary border border-border rounded-lg text-sm text-text-primary"
          aria-label="按状态码筛选"
        >
          <option value="all">全部状态</option>
          <option value="2xx">2xx 成功</option>
          <option value="4xx">4xx 客户端错误</option>
          <option value="5xx">5xx 服务端错误</option>
        </select>
        <input
          v-model="pathFilter"
          type="text"
          placeholder="按路径筛选..."
          class="flex-1 px-3 py-1.5 bg-bg-secondary border border-border rounded-lg text-sm text-text-primary"
          aria-label="按路径筛选"
        />
      </section>

      <div class="flex-1 overflow-auto bg-bg-secondary border border-border rounded-lg">
        <table class="w-full text-sm" role="table">
          <thead class="sticky top-0 bg-bg-tertiary text-text-secondary">
            <tr>
              <th class="text-left px-3 py-2 font-medium">时间</th>
              <th class="text-left px-3 py-2 font-medium">方法</th>
              <th class="text-left px-3 py-2 font-medium">路径</th>
              <th class="text-left px-3 py-2 font-medium">状态</th>
              <th class="text-left px-3 py-2 font-medium">耗时</th>
              <th class="text-left px-3 py-2 font-medium">模型</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(entry, idx) in filteredEntries"
              :key="entry.request_id || idx"
              class="border-t border-border hover:bg-bg-tertiary"
            >
              <td class="px-3 py-2 text-text-secondary whitespace-nowrap text-xs">
                {{ formatTime(entry.timestamp) }}
              </td>
              <td class="px-3 py-2 font-mono text-xs">{{ entry.method }}</td>
              <td class="px-3 py-2 font-mono text-xs truncate max-w-xs" :title="entry.path">
                {{ entry.path }}
              </td>
              <td :class="['px-3 py-2 font-mono font-bold', statusClass(entry.status)]">
                {{ entry.status }}
              </td>
              <td class="px-3 py-2 text-text-secondary text-xs">{{ entry.duration_ms }}ms</td>
              <td class="px-3 py-2 text-text-secondary text-xs truncate max-w-[150px]">
                {{ entry.model || '—' }}
              </td>
            </tr>
            <tr v-if="filteredEntries.length === 0 && !loading">
              <td colspan="6" class="px-3 py-8 text-center text-text-secondary">
                暂无审计日志
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <footer v-if="hasMore" class="mt-3 text-center">
        <button
          @click="loadLogs(false)"
          :disabled="loading"
          class="px-4 py-2 bg-bg-tertiary hover:bg-bg-primary text-text-primary rounded-lg text-sm font-medium transition-colors disabled:opacity-50"
        >
          加载更多
        </button>
      </footer>
    </div>
  </div>
</template>
