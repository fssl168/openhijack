<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { DoctorService } from '@/services'
import { useUIStore } from '@/stores/ui'
import type { DoctorResult } from '@/types'

const uiStore = useUIStore()

const results = ref<DoctorResult[]>([])
const loading = ref(false)
const lastRunAt = ref<Date | null>(null)

const summary = computed(() => {
  let pass = 0
  let warn = 0
  let fail = 0
  for (const r of results.value) {
    if (r.status === 'PASS') pass++
    else if (r.status === 'WARN') warn++
    else if (r.status === 'FAIL') fail++
  }
  return { pass, warn, fail }
})

const hasResults = computed(() => results.value.length > 0)

onMounted(() => {
  runChecks()
})

async function runChecks() {
  loading.value = true
  try {
    results.value = await DoctorService.runChecks()
    lastRunAt.value = new Date()
  } catch (e: any) {
    uiStore.showNotification(`健康检查失败: ${e?.message || '未知错误'}`, 'error', 5000)
  } finally {
    loading.value = false
  }
}

function statusClass(status: string): string {
  switch (status) {
    case 'PASS':
      return 'border-green-600 bg-green-900/30 text-green-100'
    case 'WARN':
      return 'border-yellow-600 bg-yellow-900/30 text-yellow-100'
    case 'FAIL':
      return 'border-red-600 bg-red-900/30 text-red-100'
    default:
      return 'border-gray-600 bg-gray-800/30 text-gray-100'
  }
}

function statusBadge(status: string): string {
  switch (status) {
    case 'PASS':
      return 'bg-green-700 text-green-50'
    case 'WARN':
      return 'bg-yellow-700 text-yellow-50'
    case 'FAIL':
      return 'bg-red-700 text-red-50'
    default:
      return 'bg-gray-700 text-gray-50'
  }
}

function statusLabel(status: string): string {
  switch (status) {
    case 'PASS':
      return '通过'
    case 'WARN':
      return '警告'
    case 'FAIL':
      return '失败'
    default:
      return status
  }
}
</script>

<template>
  <div class="h-full overflow-y-auto p-4 md:p-6" role="tabpanel" aria-label="健康检查面板">
    <div class="max-w-5xl mx-auto">
      <header class="flex flex-col md:flex-row md:items-center md:justify-between gap-3 mb-6">
        <div>
          <h2 class="text-xl font-bold text-text-primary">🩺 健康检查</h2>
          <p class="text-sm text-text-secondary mt-1" v-if="lastRunAt">
            上次检查: {{ lastRunAt.toLocaleTimeString() }}
          </p>
        </div>
        <button
          @click="runChecks"
          :disabled="loading"
          class="px-4 py-2 bg-primary hover:bg-primary-light text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          :aria-label="loading ? '正在检查' : '重新检查'"
        >
          <span v-if="loading">检查中...</span>
          <span v-else>🔄 重新检查</span>
        </button>
      </header>

      <section v-if="hasResults" class="grid grid-cols-3 gap-3 mb-6" aria-label="检查汇总">
        <div class="p-4 rounded-lg border border-green-600 bg-green-900/20">
          <div class="text-2xl font-bold text-green-400">{{ summary.pass }}</div>
          <div class="text-xs text-text-secondary mt-1">通过</div>
        </div>
        <div class="p-4 rounded-lg border border-yellow-600 bg-yellow-900/20">
          <div class="text-2xl font-bold text-yellow-400">{{ summary.warn }}</div>
          <div class="text-xs text-text-secondary mt-1">警告</div>
        </div>
        <div class="p-4 rounded-lg border border-red-600 bg-red-900/20">
          <div class="text-2xl font-bold text-red-400">{{ summary.fail }}</div>
          <div class="text-xs text-text-secondary mt-1">失败</div>
        </div>
      </section>

      <div v-if="loading && !hasResults" class="text-center py-12 text-text-secondary" role="status">
        正在运行健康检查...
      </div>

      <div v-else-if="!hasResults" class="text-center py-12 text-text-secondary">
        暂无检查结果，点击"重新检查"开始
      </div>

      <section v-else class="space-y-3" aria-label="检查项列表">
        <article
          v-for="item in results"
          :key="item.name"
          :class="['p-4 rounded-lg border', statusClass(item.status)]"
        >
          <div class="flex items-center justify-between gap-3">
            <h3 class="font-semibold">{{ item.name }}</h3>
            <span :class="['px-2 py-0.5 rounded text-xs font-bold', statusBadge(item.status)]">
              {{ statusLabel(item.status) }}
            </span>
          </div>
          <p class="text-sm mt-2 opacity-90">{{ item.detail }}</p>
          <div v-if="item.fix_hint" class="mt-3 p-3 bg-black/20 rounded text-sm">
            <span class="font-semibold">💡 修复建议: </span>
            <span>{{ item.fix_hint }}</span>
          </div>
        </article>
      </section>
    </div>
  </div>
</template>
