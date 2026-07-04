<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useUIStore } from '@/stores/ui'
import { GetSystemInfo, GetRuntimeEnv } from '@/utils/runtime'
import { WatcherService } from '@/services'
import CertificateManager from '@/components/CertificateManager.vue'
import type { WatcherStatus } from '@/types'

const uiStore = useUIStore()

const systemInfo = ref({
  platform: {
    os: '',
    arch: '',
    privileged: false,
    has_sudo: false,
    cap_support: false,
  },
  go_version: '',
  app_version: '1.0.0',
})

const runtimeEnv = ref({
  uid: 0,
  euid: 0,
  sudo_user: '',
  display: '',
  xauthority: '',
  home: '',
  warnings: [] as string[],
})

const watcherStatus = ref<WatcherStatus | null>(null)
const reloading = ref(false)

onMounted(() => {
  setTimeout(() => {
    loadSystemInfo()
    loadRuntimeEnv()
    loadWatcherStatus()
  }, 200)
})

async function loadWatcherStatus() {
  watcherStatus.value = await WatcherService.getStatus()
}

async function reloadConfig() {
  reloading.value = true
  const result = await WatcherService.reloadManually()
  reloading.value = false
  if (result.success) {
    uiStore.showNotification('配置已手动重载', 'success', 3000)
    await loadWatcherStatus()
  } else {
    uiStore.showNotification(result.error || '重载失败', 'error', 5000)
  }
}

async function loadSystemInfo() {
  try {
    const info = await GetSystemInfo()
    if (info) {
      systemInfo.value = info
    }
  } catch {
    systemInfo.value = {
      platform: {
        os: navigator.platform,
        arch: '',
        privileged: false,
        has_sudo: false,
        cap_support: false,
      },
      go_version: '',
      app_version: '1.0.0',
    }
  }
}

async function loadRuntimeEnv() {
  try {
    const env = await GetRuntimeEnv()
    if (env) {
      runtimeEnv.value = env
    }
  } catch {
    // ignore
  }
}
</script>

<template>
  <div class="h-full overflow-y-auto p-4 md:p-6">
    <div class="max-w-4xl mx-auto space-y-4 md:space-y-6">
      <h2 class="text-lg md:text-xl font-semibold">系统设置</h2>

      <div class="card">
        <h3 class="text-base md:text-lg font-semibold mb-3 md:mb-4">系统信息</h3>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 md:gap-4">
          <div class="bg-bg-tertiary rounded-lg p-3 md:p-4">
            <div class="text-xs md:text-sm text-text-muted mb-1">操作系统</div>
            <div class="font-medium text-sm md:text-base">{{ systemInfo.platform.os }}</div>
          </div>
          <div class="bg-bg-tertiary rounded-lg p-3 md:p-4">
            <div class="text-xs md:text-sm text-text-muted mb-1">架构</div>
            <div class="font-medium text-sm md:text-base">{{ systemInfo.platform.arch }}</div>
          </div>
          <div class="bg-bg-tertiary rounded-lg p-3 md:p-4">
            <div class="text-xs md:text-sm text-text-muted mb-1">Go 版本</div>
            <div class="font-mono text-xs md:text-sm">{{ systemInfo.go_version || 'N/A' }}</div>
          </div>
          <div class="bg-bg-tertiary rounded-lg p-3 md:p-4">
            <div class="text-xs md:text-sm text-text-muted mb-1">应用版本</div>
            <div class="font-medium text-sm md:text-base">v{{ systemInfo.app_version }}</div>
          </div>
        </div>
      </div>

      <div class="card">
        <h3 class="text-base md:text-lg font-semibold mb-3 md:mb-4">权限状态</h3>
        <div class="space-y-2 md:space-y-3">
          <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between py-2 md:py-3 border-b border-border gap-2">
            <div>
              <div class="font-medium text-sm md:text-base">管理员权限</div>
              <div class="text-xs md:text-sm text-text-muted">当前是否以 root/Administrator 运行</div>
            </div>
            <span class="status-badge" :class="systemInfo.platform.privileged ? 'status-running' : 'status-stopped'">
              {{ systemInfo.platform.privileged ? '是' : '否' }}
            </span>
          </div>
          <div v-if="systemInfo.platform.cap_support" class="flex flex-col sm:flex-row items-start sm:items-center justify-between py-2 md:py-3 border-b border-border gap-2">
            <div>
              <div class="font-medium text-sm md:text-base">Capabilities 支持</div>
              <div class="text-xs md:text-sm text-text-muted">Linux cap_net_bind_service</div>
            </div>
            <span class="status-badge status-running">
              可用
            </span>
          </div>
        </div>
      </div>

      <div v-if="runtimeEnv.warnings.length > 0" class="card bg-yellow-900/20 border-yellow-700">
        <h3 class="text-base md:text-lg font-semibold mb-3 md:mb-4 text-yellow-300">⚠️ 运行环境警告</h3>
        <div class="space-y-1 md:space-y-2">
          <div v-for="(warning, i) in runtimeEnv.warnings" :key="i" class="text-xs md:text-sm text-yellow-200 flex gap-2">
            <span>•</span>
            <span>{{ warning }}</span>
          </div>
        </div>
      </div>

      <div class="card">
        <h3 class="text-base md:text-lg font-semibold mb-3 md:mb-4">运行环境详情</h3>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 md:gap-4">
          <div class="bg-bg-tertiary rounded-lg p-3 md:p-4">
            <div class="text-xs md:text-sm text-text-muted mb-1">UID / EUID</div>
            <div class="font-mono text-xs md:text-sm">{{ runtimeEnv.uid }} / {{ runtimeEnv.euid }}</div>
          </div>
          <div class="bg-bg-tertiary rounded-lg p-3 md:p-4">
            <div class="text-xs md:text-sm text-text-muted mb-1">SUDO_USER</div>
            <div class="font-medium text-xs md:text-sm">{{ runtimeEnv.sudo_user || '(无)' }}</div>
          </div>
          <div class="bg-bg-tertiary rounded-lg p-3 md:p-4">
            <div class="text-xs md:text-sm text-text-muted mb-1">DISPLAY</div>
            <div class="font-mono text-xs md:text-sm">{{ runtimeEnv.display || '(未设置)' }}</div>
          </div>
          <div class="bg-bg-tertiary rounded-lg p-3 md:p-4">
            <div class="text-xs md:text-sm text-text-muted mb-1">HOME</div>
            <div class="font-mono text-xs truncate">{{ runtimeEnv.home || '(未知)' }}</div>
          </div>
        </div>
      </div>

      <div class="card">
        <h3 class="text-base md:text-lg font-semibold mb-3 md:mb-4">配置热重载</h3>
        <div class="space-y-3">
          <div class="flex items-center justify-between py-2 border-b border-border">
            <div>
              <div class="font-medium text-sm md:text-base">Watcher 状态</div>
              <div class="text-xs md:text-sm text-text-muted">配置文件监听服务</div>
            </div>
            <span
              class="status-badge"
              :class="watcherStatus?.running ? 'status-running' : 'status-stopped'"
            >
              {{ watcherStatus?.running ? '运行中' : '未运行' }}
            </span>
          </div>
          <div v-if="watcherStatus?.last_reload" class="flex items-center justify-between py-2 border-b border-border">
            <div>
              <div class="font-medium text-sm md:text-base">上次重载</div>
              <div class="text-xs md:text-sm text-text-muted">配置文件变更后自动重载时间</div>
            </div>
            <span class="font-mono text-xs md:text-sm">{{ watcherStatus.last_reload }}</span>
          </div>
          <div v-if="watcherStatus?.last_error" class="bg-red-900/20 border border-red-700 rounded-lg p-3">
            <div class="font-medium text-sm text-red-300 mb-1">上次错误</div>
            <div class="text-xs text-red-200 font-mono break-all">{{ watcherStatus.last_error }}</div>
          </div>
          <button
            @click="reloadConfig"
            :disabled="reloading || !watcherStatus?.running"
            class="px-4 py-2 bg-primary hover:bg-primary-light text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {{ reloading ? '重载中...' : '🔄 手动重载配置' }}
          </button>
        </div>
      </div>

      <CertificateManager />

      <div class="card">
        <h3 class="text-base md:text-lg font-semibold mb-3 md:mb-4">关于</h3>
        <div class="space-y-3 md:space-y-4 text-xs md:text-sm">
          <div>
            <h4 class="font-medium mb-1 text-sm md:text-base">OpenHijack</h4>
            <p class="text-text-muted">
              本地 HTTPS 代理服务器，用于在开发环境中拦截和转发 API 请求。
            </p>
          </div>
          <div>
            <h4 class="font-medium mb-1 text-sm md:text-base">技术栈</h4>
            <div class="flex flex-wrap gap-1 md:gap-2">
              <span class="px-2 py-1 bg-bg-tertiary rounded text-xs">Go</span>
              <span class="px-2 py-1 bg-bg-tertiary rounded text-xs">Wails</span>
              <span class="px-2 py-1 bg-bg-tertiary rounded text-xs">Vue 3</span>
              <span class="px-2 py-1 bg-bg-tertiary rounded text-xs">Tailwind CSS</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>