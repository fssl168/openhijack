<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import Dashboard from './views/Dashboard.vue'
import ConfigManager from './views/ConfigManager.vue'
import LogViewer from './views/LogViewer.vue'
import Settings from './views/Settings.vue'
import { useUIStore } from './stores/ui'
import { useProxyStore } from './stores/proxy'

const uiStore = useUIStore()
const proxyStore = useProxyStore()

const proxyStatus = computed(() => proxyStore.running)

onMounted(() => {
  proxyStore.startPolling()
})

onUnmounted(() => {
  proxyStore.stopPolling()
})

const navItems = [
  { view: 'dashboard' as const, label: '📋 仪表盘' },
  { view: 'configs' as const, label: '⚙️ 配置管理' },
  { view: 'logs' as const, label: '📊 日志' },
  { view: 'settings' as const, label: '❓ 设置' },
]
</script>

<template>
  <div class="h-screen flex flex-col bg-bg-primary text-text-primary">
    <header class="flex items-center justify-between px-6 py-3 bg-bg-secondary border-b border-border">
      <div class="flex items-center gap-3">
        <div class="w-8 h-8 bg-primary rounded-lg flex items-center justify-center">
          <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
        </div>
        <h1 class="text-lg font-bold">OpenHijack</h1>
      </div>

      <nav class="flex items-center gap-1">
        <button
          v-for="item in navItems"
          :key="item.view"
          @click="uiStore.setView(item.view)"
          :class="[
            'px-4 py-2 rounded-lg text-sm font-medium transition-colors',
            uiStore.currentView === item.view
              ? 'bg-primary text-white'
              : 'text-text-secondary hover:bg-bg-tertiary hover:text-text-primary'
          ]"
        >
          {{ item.label }}
        </button>
      </nav>

      <div class="flex items-center gap-2">
        <span class="status-badge" :class="proxyStatus ? 'status-running' : 'status-stopped'">
          <span
            class="w-2 h-2 rounded-full"
            :class="{
              'bg-green-400 animate-pulse': proxyStore.starting,
              'bg-yellow-400 animate-pulse': proxyStore.stopping,
              'bg-green-400': proxyStore.running && !proxyStore.starting,
              'bg-red-400': !proxyStore.running,
            }"
          ></span>
          {{ proxyStore.starting ? '启动中...' : proxyStore.stopping ? '停止中...' : proxyStatus ? '运行中' : '已停止' }}
        </span>
      </div>
    </header>

    <main class="flex-1 overflow-hidden">
      <Dashboard v-show="uiStore.currentView === 'dashboard'" />
      <ConfigManager v-show="uiStore.currentView === 'configs'" />
      <LogViewer v-show="uiStore.currentView === 'logs'" />
      <Settings v-show="uiStore.currentView === 'settings'" />
    </main>

    <div class="fixed bottom-4 right-4 flex flex-col gap-2 z-50">
      <div
        v-for="notif in uiStore.notifications"
        :key="notif.id"
        @click="uiStore.dismissNotification(notif.id)"
        :class="[
          'px-4 py-3 rounded-lg shadow-lg border cursor-pointer max-w-sm',
          notif.type === 'success' ? 'bg-green-900/80 border-green-700 text-green-100' : '',
          notif.type === 'error' ? 'bg-red-900/80 border-red-700 text-red-100' : '',
          notif.type === 'info' ? 'bg-blue-900/80 border-blue-700 text-blue-100' : '',
          notif.type === 'warn' ? 'bg-yellow-900/80 border-yellow-700 text-yellow-100' : '',
        ]"
      >
        {{ notif.message }}
      </div>
    </div>
  </div>
</template>
