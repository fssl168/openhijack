<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useUIStore } from '@/stores/ui'
import { GetSystemInfo, InstallCACert, UninstallCACert } from '@/utils/runtime'

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

const caInstalled = ref(false)
const loading = ref(false)

onMounted(async () => {
  loadSystemInfo()
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

async function handleInstallCert() {
  loading.value = true
  try {
    const err = await InstallCACert()
    if (err) {
      uiStore.showNotification(`安装失败: ${err}`, 'error')
    } else {
      uiStore.showNotification('CA 证书安装成功', 'success')
      caInstalled.value = true
    }
  } catch (e: any) {
    uiStore.showNotification(`安装失败: ${e?.message}`, 'error')
  } finally {
    loading.value = false
  }
}

async function handleUninstallCert() {
  loading.value = true
  try {
    await UninstallCACert()
    uiStore.showNotification('CA 证书已卸载', 'info')
    caInstalled.value = false
  } catch (e: any) {
    uiStore.showNotification(`卸载失败: ${e?.message}`, 'error')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="h-full overflow-y-auto p-6">
    <div class="max-w-4xl mx-auto space-y-6">
      <h2 class="text-xl font-semibold">系统设置</h2>

      <div class="card">
        <h3 class="text-lg font-semibold mb-4">系统信息</h3>
        <div class="grid grid-cols-2 gap-4">
          <div class="bg-bg-tertiary rounded-lg p-4">
            <div class="text-sm text-text-muted mb-1">操作系统</div>
            <div class="font-medium">{{ systemInfo.platform.os }}</div>
          </div>
          <div class="bg-bg-tertiary rounded-lg p-4">
            <div class="text-sm text-text-muted mb-1">架构</div>
            <div class="font-medium">{{ systemInfo.platform.arch }}</div>
          </div>
          <div class="bg-bg-tertiary rounded-lg p-4">
            <div class="text-sm text-text-muted mb-1">Go 版本</div>
            <div class="font-mono">{{ systemInfo.go_version || 'N/A' }}</div>
          </div>
          <div class="bg-bg-tertiary rounded-lg p-4">
            <div class="text-sm text-text-muted mb-1">应用版本</div>
            <div class="font-medium">v{{ systemInfo.app_version }}</div>
          </div>
        </div>
      </div>

      <div class="card">
        <h3 class="text-lg font-semibold mb-4">权限状态</h3>
        <div class="space-y-3">
          <div class="flex items-center justify-between py-3 border-b border-border">
            <div>
              <div class="font-medium">管理员权限</div>
              <div class="text-sm text-text-muted">当前是否以 root/Administrator 运行</div>
            </div>
            <span class="status-badge" :class="systemInfo.platform.privileged ? 'status-running' : 'status-stopped'">
              {{ systemInfo.platform.privileged ? '是' : '否' }}
            </span>
          </div>
          <div v-if="systemInfo.platform.cap_support" class="flex items-center justify-between py-3 border-b border-border">
            <div>
              <div class="font-medium">Capabilities 支持</div>
              <div class="text-sm text-text-muted">Linux cap_net_bind_service</div>
            </div>
            <span class="status-badge status-running">
              可用
            </span>
          </div>
        </div>
      </div>

      <div class="card">
        <h3 class="text-lg font-semibold mb-4">CA 证书管理</h3>
        <p class="text-text-muted mb-4">
          OpenHijack 通过本地 HTTPS 代理工作，需要将自签名 CA 证书安装到系统信任库。
        </p>
        <div class="flex items-center gap-3">
          <button
            @click="handleInstallCert"
            :disabled="loading"
            class="btn-success"
          >
            安装 CA 证书
          </button>
          <button
            @click="handleUninstallCert"
            :disabled="loading"
            class="btn-error"
          >
            卸载 CA 证书
          </button>
        </div>
      </div>

      <div class="card">
        <h3 class="text-lg font-semibold mb-4">关于</h3>
        <div class="space-y-4 text-sm">
          <div>
            <h4 class="font-medium mb-1">OpenHijack</h4>
            <p class="text-text-muted">
              本地 HTTPS 代理服务器，用于在开发环境中拦截和转发 API 请求。
            </p>
          </div>
          <div>
            <h4 class="font-medium mb-1">技术栈</h4>
            <div class="flex flex-wrap gap-2">
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