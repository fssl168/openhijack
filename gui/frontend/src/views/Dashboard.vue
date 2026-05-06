<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useProxyStore } from '@/stores/proxy'
import { useConfigStore } from '@/stores/config'
import { useUIStore } from '@/stores/ui'

const proxyStore = useProxyStore()
const configStore = useConfigStore()
const uiStore = useUIStore()

const port = ref(443)
const loading = ref(false)
const recentLogs = ref<string[]>([])

onMounted(async () => {
  await proxyStore.getStatus()
  await configStore.loadConfigs()
  loadRecentLogs()
})

watch(() => configStore.activeConfig, async (newConfig) => {
  if (newConfig) {
    await proxyStore.getStatus()
    loadRecentLogs()
  }
}, { immediate: false })

async function handleStart() {
  if (!configStore.activeConfig) {
    uiStore.showNotification('请先选择或创建一个配置', 'warn')
    return
  }

  loading.value = true
  const err = await proxyStore.start(configStore.activeConfig, port.value)
  loading.value = false

  if (err) {
    uiStore.showNotification(`启动失败: ${err}`, 'error')
  } else {
    uiStore.showNotification('代理服务已启动', 'success')
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

function loadRecentLogs() {
  recentLogs.value = proxyStore.logs.slice(-5).map(l => l.raw)
}
</script>

<template>
  <div class="h-full overflow-y-auto p-6">
    <div class="max-w-6xl mx-auto space-y-6">
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div class="card col-span-2">
          <div class="flex items-center justify-between mb-6">
            <h2 class="text-xl font-semibold">服务状态</h2>
            <span class="status-badge" :class="proxyStore.running ? 'status-running' : 'status-stopped'">
              <span class="w-2 h-2 rounded-full" :class="proxyStore.running ? 'bg-green-400' : 'bg-red-400'"></span>
              {{ proxyStore.running ? '运行中' : '已停止' }}
            </span>
          </div>

          <div class="grid grid-cols-2 gap-4 mb-6">
            <div class="bg-bg-tertiary rounded-lg p-4">
              <div class="text-sm text-text-muted mb-1">监听端口</div>
              <div class="text-2xl font-mono">{{ proxyStore.port || port }}</div>
            </div>
            <div class="bg-bg-tertiary rounded-lg p-4">
              <div class="text-sm text-text-muted mb-1">运行时间</div>
              <div class="text-2xl font-mono">{{ proxyStore.uptime || '未启动' }}</div>
            </div>
            <div class="bg-bg-tertiary rounded-lg p-4">
              <div class="text-sm text-text-muted mb-1">当前模型</div>
              <div class="text-lg">{{ proxyStore.model || '未配置' }}</div>
            </div>
            <div class="bg-bg-tertiary rounded-lg p-4">
              <div class="text-sm text-text-muted mb-1">Provider</div>
              <div class="text-lg">{{ proxyStore.provider || '未配置' }}</div>
            </div>
          </div>

          <div class="flex items-center gap-4">
            <div class="flex-1">
              <label class="block text-sm text-text-muted mb-2">监听端口</label>
              <input
                v-model.number="port"
                type="number"
                :disabled="proxyStore.running"
                class="input-field"
                placeholder="443"
              />
            </div>
            <button
              v-if="!proxyStore.running"
              @click="handleStart"
              :disabled="loading"
              class="btn-success px-8"
            >
              ▶ 启动
            </button>
            <button
              v-else
              @click="handleStop"
              :disabled="loading"
              class="btn-error px-8"
            >
              ■ 停止
            </button>
          </div>
        </div>

        <div class="card">
          <h3 class="text-lg font-semibold mb-4">快捷操作</h3>
          <div class="space-y-3">
            <button
              @click="uiStore.setView('configs')"
              class="w-full btn-outline text-left"
            >
              ⚙️ 管理配置
            </button>
            <button
              @click="uiStore.setView('logs')"
              class="w-full btn-outline text-left"
            >
              📊 查看日志
            </button>
            <button
              @click="uiStore.setView('settings')"
              class="w-full btn-outline text-left"
            >
              ❓ 系统设置
            </button>
          </div>

          <div class="mt-6 pt-6 border-t border-border">
            <h4 class="text-sm text-text-muted mb-2">系统信息</h4>
            <div class="text-sm space-y-1">
              <div class="flex justify-between">
                <span class="text-text-muted">版本</span>
                <span>1.0.0</span>
              </div>
              <div class="flex justify-between">
                <span class="text-text-muted">架构</span>
                <span>{{ proxyStore.running ? 'Active' : 'Idle' }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-lg font-semibold">最近日志</h3>
          <button @click="uiStore.setView('logs')" class="text-sm text-primary-light hover:text-primary">
            查看全部 →
          </button>
        </div>
        <div class="bg-bg-primary rounded-lg p-4 font-mono text-sm max-h-48 overflow-y-auto">
          <div v-if="proxyStore.logs.length === 0" class="text-text-muted">
            暂无日志记录...
          </div>
          <div
            v-for="(log, i) in proxyStore.logs.slice(-8)"
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
