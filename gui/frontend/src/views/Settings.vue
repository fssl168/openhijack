<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useUIStore } from '@/stores/ui'
import { GetSystemInfo, GetRuntimeEnv } from '@/utils/runtime'
import CertificateManager from '@/components/CertificateManager.vue'

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

onMounted(() => {
  setTimeout(() => {
    loadSystemInfo()
    loadRuntimeEnv()
  }, 200)
})

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