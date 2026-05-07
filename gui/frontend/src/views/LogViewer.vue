<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onUnmounted, watch } from 'vue'
import { useProxyStore } from '@/stores/proxy'
import type { LogLevel } from '@/types'

const proxyStore = useProxyStore()

const filterLevel = ref<LogLevel | 'all'>('all')
const searchQuery = ref('')
const autoScroll = ref(true)
const logContainer = ref<HTMLElement | null>(null)
const loadMoreTrigger = ref<HTMLElement | null>(null)
const isUserScrolling = ref(false)
let scrollTimer: ReturnType<typeof setTimeout> | null = null

const PAGE_SIZE = 100
const visibleStartIndex = ref(0)
const visibleEndIndex = ref(PAGE_SIZE)

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

const visibleLogs = computed(() => {
  if (autoScroll.value && !isUserScrolling.value) {
    const total = filteredLogs.value.length
    const start = Math.max(0, total - PAGE_SIZE)
    return filteredLogs.value.slice(start, total)
  }
  return filteredLogs.value.slice(visibleStartIndex.value, visibleEndIndex.value)
})

const hasMoreLogs = computed(() => {
  if (autoScroll.value && !isUserScrolling.value) {
    return false
  }
  return filteredLogs.value.length > visibleEndIndex.value
})

onMounted(() => {
  setupLogListener()
  setupIntersectionObserver()
})

onUnmounted(() => {
  window.removeEventListener('log:incoming', handleLogIncoming as any)
  if (scrollTimer) clearTimeout(scrollTimer)
})

function setupIntersectionObserver() {
  if (!('IntersectionObserver' in window)) return

  const observer = new IntersectionObserver((entries) => {
    if (entries[0]?.isIntersecting && hasMoreLogs.value && !autoScroll.value) {
      loadMoreLogs()
    }
  }, { rootMargin: '200px' })

  watch(loadMoreTrigger, (el) => {
    if (el) {
      observer.observe(el)
    }
  })
}

function loadMoreLogs() {
  const nextEndIndex = Math.min(
    visibleEndIndex.value + PAGE_SIZE,
    filteredLogs.value.length
  )
  visibleEndIndex.value = nextEndIndex
}

function resetPagination() {
  visibleStartIndex.value = 0
  visibleEndIndex.value = PAGE_SIZE
}

function scrollToBottom() {
  if (!logContainer.value) return
  logContainer.value.scrollTop = logContainer.value.scrollHeight
}

function handleUserScroll() {
  if (!logContainer.value || !autoScroll.value) return

  const el = logContainer.value
  const isAtBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 50

  if (!isAtBottom) {
    isUserScrolling.value = true
    if (scrollTimer) clearTimeout(scrollTimer)
    scrollTimer = setTimeout(() => {
      isUserScrolling.value = false
    }, 3000)
  } else {
    isUserScrolling.value = false
    if (scrollTimer) {
      clearTimeout(scrollTimer)
      scrollTimer = null
    }
  }
}

function setupLogListener() {
  window.addEventListener('log:incoming', handleLogIncoming as any)

  if (logContainer.value) {
    logContainer.value.addEventListener('scroll', handleUserScroll)
  }

  watch(logContainer, (el) => {
    if (el) {
      el.addEventListener('scroll', handleUserScroll)
      scrollToBottom()
    }
  })
}

function handleLogIncoming() {
  if (!autoScroll.value || isUserScrolling.value) return

  nextTick(() => {
    scrollToBottom()
  })
}

watch([() => proxyStore.logs.length], () => {
  if (autoScroll.value && !isUserScrolling.value) {
    nextTick(() => {
      scrollToBottom()
    })
  }
}, { flush: 'post' })

function clearLogs() {
  proxyStore.clearLogs()
  resetPagination()
  nextTick(() => {
    scrollToBottom()
  })
}

watch(filterLevel, () => {
  resetPagination()
  if (autoScroll.value) {
    nextTick(scrollToBottom)
  }
})

watch(searchQuery, () => {
  resetPagination()
  if (autoScroll.value) {
    nextTick(scrollToBottom)
  }
})
</script>

<template>
  <div class="h-full flex flex-col p-4 md:p-6 min-h-0">
    <div class="max-w-6xl mx-auto w-full flex-1 flex flex-col min-h-0">
      <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between mb-3 md:mb-4 gap-2 shrink-0">
        <h2 class="text-lg md:text-xl font-semibold">日志查看器</h2>
        <div class="flex items-center gap-2 md:gap-3 w-full sm:w-auto">
          <label class="flex items-center gap-2 text-xs md:text-sm text-text-muted cursor-pointer select-none">
            <input type="checkbox" v-model="autoScroll" class="rounded" />
            自动滚动
          </label>
          <button @click="clearLogs" class="btn-outline px-2 md:px-3 py-1 text-xs md:text-sm w-full sm:w-auto shrink-0">
            清空
          </button>
        </div>
      </div>

      <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-2 md:gap-3 mb-3 md:mb-4 shrink-0">
        <div class="flex-1 relative">
          <input
            v-model="searchQuery"
            placeholder="搜索日志..."
            class="input-field pl-8 md:pl-10 text-xs md:text-sm"
          />
          <svg class="w-4 h-4 md:w-5 md:h-5 text-text-muted absolute left-2 md:left-3 top-1/2 -translate-y-1/2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </div>

        <select v-model="filterLevel" class="input-field w-full sm:w-28 md:w-32 text-xs md:text-sm shrink-0">
          <option value="all">全部</option>
          <option value="info">INFO</option>
          <option value="warn">WARN</option>
          <option value="error">ERROR</option>
        </select>
      </div>

      <div
        ref="logContainer"
        class="flex-1 bg-bg-primary rounded-lg border border-border p-2 md:p-4 font-mono text-xs md:text-sm overflow-y-auto min-h-0 relative"
        @scroll="handleUserScroll"
      >
        <div v-if="filteredLogs.length === 0" class="text-center py-8 md:py-12 text-text-muted">
          {{ searchQuery ? '没有匹配的日志' : '暂无日志记录...' }}
        </div>
        <template v-else>
          <div
            v-for="(log, i) in visibleLogs"
            :key="log.raw + '_' + i"
            class="log-line py-1 px-1 md:px-2 rounded hover:bg-bg-secondary transition-colors text-xs md:text-sm whitespace-pre-wrap break-words"
            :class="{
              'log-info': log.level === 'info',
              'log-warn': log.level === 'warn',
              'log-error': log.level === 'error',
            }"
          >
            <span class="text-text-muted mr-2 md:mr-4 inline-block shrink-0">{{ log.timestamp }}</span>
            <span
              v-if="log.level !== 'info'"
              class="text-xs px-1 py-0.5 rounded mr-1 md:mr-2 inline-block shrink-0"
              :class="{
                'bg-yellow-900/50 text-yellow-300': log.level === 'warn',
                'bg-red-900/50 text-red-300': log.level === 'error',
              }"
            >
              {{ log.level.toUpperCase() }}
            </span>
            <span class="inline-wrap">{{ log.message }}</span>
          </div>

          <div
            v-if="hasMoreLogs"
            ref="loadMoreTrigger"
            class="text-center py-4 text-text-muted cursor-pointer hover:text-primary-light transition-colors shrink-0"
            @click="loadMoreLogs"
          >
            加载更多日志 ({{ filteredLogs.length - visibleEndIndex }} 条剩余)
          </div>
        </template>
      </div>

      <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between mt-2 md:mt-3 text-xs md:text-sm text-text-muted gap-1 shrink-0">
        <span>{{ filteredLogs.length }} 条日志</span>
        <span>最大保留: {{ proxyStore.maxLogs }} 条</span>
      </div>
    </div>
  </div>
</template>
