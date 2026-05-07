<script setup lang="ts">
import { ref, computed } from 'vue'
import { useProxyStore } from '@/stores/proxy'
import { useConfigStore } from '@/stores/config'
import { useUIStore } from '@/stores/ui'

const proxyStore = useProxyStore()
const configStore = useConfigStore()
const uiStore = useUIStore()

const port = ref(8443)
const loading = ref(false)

const canStart = computed(() => !proxyStore.running && !proxyStore.starting && !proxyStore.stopping)
const canStop = computed(() => (proxyStore.running || proxyStore.starting) && !proxyStore.stopping)
const isBusy = computed(() => proxyStore.starting || proxyStore.stopping || loading.value)
const startDisabled = computed(() => isBusy.value || !configStore.activeConfig)

const recentLogs = computed(() => proxyStore.logs.slice(-8))

async function handleStart() {
  if (!configStore.activeConfig) {
    uiStore.showNotification('请先选择或创建一个配置', 'warn')
    return
  }

  loading.value = true
  const err = await proxyStore.start(configStore.activeConfig, port.value)
  loading.value = false

  if (err) {
    if (err.includes('root') || err.includes('permission') || err.includes('权限')) {
      uiStore.showNotification(`${err}（已自动切换到端口 8443）`, 'error', 8000)
      if (port.value < 1024) {
        port.value = 8443
      }
    } else {
      uiStore.showNotification(`启动失败: ${err}`, 'error')
    }
  } else {
    uiStore.showNotification(`代理服务已启动 (端口 ${port.value})`, 'success')
  }
}

async function handleStop() {
  loading.value = true
  const err = await proxyStore.stop()
  loading.value = false

  if (err) {
    uiStore.showNotification(`停止失败: ${err}`, 'error')
  } else {
    uiStore.showNotification('代理服务已停止', 'info')
  }
}
</script>

<template>
  <div class="h-full overflow-y-auto p-4 md:p-6">
    <div class="max-w-6xl mx-auto space-y-4 md:space-y-6">
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4 md:gap-6">
        <div class="card col-span-1 lg:col-span-2">
          <div class="flex flex-col sm:flex-row sm:items-center justify-between mb-4 md:mb-6 gap-2">
            <h2 class="text-lg md:text-xl font-semibold">服务状态</h2>
            <span class="status-badge" :class="proxyStore.running ? 'status-running' : 'status-stopped'">
              <span
                class="w-2 h-2 rounded-full"
                :class="{
                  'bg-green-400 animate-pulse': proxyStore.starting,
                  'bg-yellow-400 animate-pulse': proxyStore.stopping,
                  'bg-green-400': proxyStore.running && !proxyStore.starting,
                  'bg-red-400': !proxyStore.running,
                }"
              ></span>
              {{ proxyStore.starting ? '启动中...' : proxyStore.stopping ? '停止中...' : proxyStore.running ? '运行中' : '已停止' }}
            </span>
          </div>

          <div class="grid grid-cols-2 lg:grid-cols-2 gap-3 md:gap-4 mb-4 md:mb-6">
            <div class="bg-bg-tertiary rounded-lg p-3 md:p-4">
              <div class="text-xs md:text-sm text-text-muted mb-1">监听端口</div>
              <div class="text-lg md:text-2xl font-mono">{{ proxyStore.port || port }}</div>
            </div>
            <div class="bg-bg-tertiary rounded-lg p-3 md:p-4">
              <div class="text-xs md:text-sm text-text-muted mb-1">运行时间</div>
              <div class="text-lg md:text-2xl font-mono">{{ proxyStore.uptime || '未启动' }}</div>
            </div>
            <div class="bg-bg-tertiary rounded-lg p-3 md:p-4">
              <div class="text-xs md:text-sm text-text-muted mb-1">当前模型</div>
              <div class="text-base md:text-lg">{{ proxyStore.model || '未配置' }}</div>
            </div>
            <div class="bg-bg-tertiary rounded-lg p-3 md:p-4">
              <div class="text-xs md:text-sm text-text-muted mb-1">Provider</div>
              <div class="text-base md:text-lg">{{ proxyStore.provider || '未配置' }}</div>
            </div>
          </div>

          <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-3 md:gap-4">
            <div class="flex-1 w-full sm:w-auto">
              <label class="block text-xs md:text-sm text-text-muted mb-2">监听端口</label>
              <input
                v-model.number="port"
                type="number"
                :disabled="proxyStore.running"
                class="input-field"
                placeholder="443"
              />
            </div>
            <div class="flex flex-col sm:flex-row gap-2 sm:gap-4 w-full sm:w-auto pt-6 md:pt-0">
              <button
                v-if="canStart"
                @click="handleStart"
                :disabled="startDisabled"
                :class="['btn-success px-6 md:px-8 flex-1 sm:flex-initial', { 'opacity-50 cursor-not-allowed': startDisabled }]"
                :title="!configStore.activeConfig ? '请先在配置管理中选择一个配置' : ''"
              >
                ▶ 启动
              </button>
              <button
                v-else-if="proxyStore.starting"
                disabled
                class="btn-success px-6 md:px-8 opacity-70 cursor-wait flex-1 sm:flex-initial"
              >
                ⏳ 启动中...
              </button>
              <button
                v-else-if="canStop"
                @click="handleStop"
                :disabled="isBusy"
                class="btn-error px-6 md:px-8 flex-1 sm:flex-initial"
              >
                ■ 停止
              </button>
              <button
                v-else-if="proxyStore.stopping"
                disabled
                class="btn-error px-6 md:px-8 opacity-70 cursor-wait flex-1 sm:flex-initial"
              >
                ⏳ 停止中...
              </button>
            </div>
          </div>
        </div>

        <div class="card order-first lg:order-last">
          <h3 class="text-base md:text-lg font-semibold mb-3 md:mb-4">快捷操作</h3>
          <div class="space-y-2 md:space-y-3">
            <button
              @click="uiStore.setView('configs')"
              class="w-full btn-outline text-left py-3 md:py-2"
            >
              ⚙️ 管理配置
            </button>
            <button
              @click="uiStore.setView('logs')"
              class="w-full btn-outline text-left py-3 md:py-2"
            >
              📊 查看日志
            </button>
            <button
              @click="uiStore.setView('settings')"
              class="w-full btn-outline text-left py-3 md:py-2"
            >
              ❓ 系统设置
            </button>
          </div>

          <div class="mt-4 md:mt-6 pt-4 md:pt-6 border-t border-border">
            <h4 class="text-xs md:text-sm text-text-muted mb-2">系统信息</h4>
            <div class="text-xs md:text-sm space-y-1">
              <div class="flex justify-between">
                <span class="text-text-muted">版本</span>
                <span>1.0.0</span>
              </div>
              <div class="flex justify-between">
                <span class="text-text-muted">状态</span>
                <span>{{ proxyStore._state }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between mb-3 md:mb-4 gap-2">
          <h3 class="text-base md:text-lg font-semibold">最近日志</h3>
          <button @click="uiStore.setView('logs')" class="text-xs md:text-sm text-primary-light hover:text-primary">
            查看全部 →
          </button>
        </div>
        <div class="bg-bg-primary rounded-lg p-3 md:p-4 font-mono text-xs md:text-sm max-h-40 md:max-h-48 overflow-y-auto">
          <div v-if="recentLogs.length === 0" class="text-text-muted">
            暂无日志记录...
          </div>
          <div
            v-for="(log, i) in recentLogs"
            :key="i"
            class="log-line"
            :class="{
              'log-info': log.level === 'info',
              'log-warn': log.level === 'warn',
              'log-error': log.level === 'error',
            }"
          >
            {{ log.raw }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
