<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import Dashboard from './views/Dashboard.vue'
import ConfigManager from './views/ConfigManager.vue'
import LogViewer from './views/LogViewer.vue'
import Settings from './views/Settings.vue'
import Doctor from './views/Doctor.vue'
import AuditLog from './views/AuditLog.vue'
import OnboardingWizard from './components/OnboardingWizard.vue'
import { useUIStore } from './stores/ui'
import { useProxyStore } from './stores/proxy'
import { useOnboardingStore } from './stores/onboarding'
import { isRuntimeReady } from './utils/runtime'
import { WatcherService } from './services'

const uiStore = useUIStore()
const proxyStore = useProxyStore()
const onboardingStore = useOnboardingStore()

const proxyStatus = computed(() => proxyStore.running)

onMounted(async () => {
  await onboardingStore.initialize()

  setTimeout(() => {
    if (isRuntimeReady()) {
      proxyStore.startPolling()
    }
  }, 300)

  // Phase D4: subscribe to config:reloaded events so the UI
  // refreshes status and notifies the user after every hot-reload.
  WatcherService.onConfigReloaded((payload) => {
    if (payload.last_error) {
      uiStore.showNotification(`配置重载失败: ${payload.last_error}`, 'error', 5000)
    } else if (payload.last_reload) {
      uiStore.showNotification('配置已热重载', 'success', 3000)
    }
    proxyStore.getStatus()
  })
})

onUnmounted(() => {
  proxyStore.stopPolling()
})

const navItems = [
  { view: 'dashboard' as const, label: '仪表盘', icon: '📋', ariaLabel: '切换到仪表盘视图' },
  { view: 'configs' as const, label: '配置管理', icon: '⚙️', ariaLabel: '切换到配置管理视图' },
  { view: 'logs' as const, label: '日志', icon: '📊', ariaLabel: '切换到日志查看器' },
  { view: 'doctor' as const, label: '健康检查', icon: '🩺', ariaLabel: '切换到健康检查视图' },
  { view: 'audit' as const, label: '审计日志', icon: '📜', ariaLabel: '切换到审计日志视图' },
  { view: 'settings' as const, label: '设置', icon: '❓', ariaLabel: '切换到系统设置' },
]

function handleNavClick(view: string) {
  uiStore.setView(view as any)
}
</script>

<template>
  <div class="h-screen flex flex-col bg-bg-primary text-text-primary" role="application" aria-label="OpenHijack 应用">
    <a href="#main-content" class="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:bg-primary-light focus:text-white focus:px-4 focus:py-2 focus:rounded focus:z-[10000]">
      跳转到主要内容
    </a>

    <OnboardingWizard />

    <header class="flex items-center justify-between px-4 md:px-6 py-3 bg-bg-secondary border-b border-border" role="banner">
      <div class="flex items-center gap-3">
        <div class="w-8 h-8 bg-primary rounded-lg flex items-center justify-center" aria-hidden="true">
          <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
        </div>
        <h1 class="text-base md:text-lg font-bold">OpenHijack</h1>
      </div>

      <nav class="flex items-center gap-1" role="navigation" aria-label="主导航">
        <button
          v-for="item in navItems"
          :key="item.view"
          @click="handleNavClick(item.view)"
          :aria-label="item.ariaLabel"
          :aria-current="uiStore.currentView === item.view ? 'page' : undefined"
          :class="[
            'px-2 md:px-4 py-2 rounded-lg text-xs md:text-sm font-medium transition-colors',
            uiStore.currentView === item.view
              ? 'bg-primary text-white'
              : 'text-text-secondary hover:bg-bg-tertiary hover:text-text-primary'
          ]"
          role="tab"
        >
          <span class="hidden sm:inline">{{ item.icon }} {{ item.label }}</span>
          <span class="sm:hidden">{{ item.icon }}</span>
        </button>
      </nav>

      <div class="flex items-center gap-2" role="status" aria-live="polite" aria-label="代理服务状态">
        <span class="status-badge" :class="proxyStatus ? 'status-running' : 'status-stopped'" role="status">
          <span
            class="w-2 h-2 rounded-full"
            :class="{
              'bg-green-400 animate-pulse': proxyStore.starting,
              'bg-yellow-400 animate-pulse': proxyStore.stopping,
              'bg-green-400': proxyStore.running && !proxyStore.starting,
              'bg-red-400': !proxyStore.running,
            }"
            aria-hidden="true"
          ></span>
          <span class="sr-only">代理状态：</span>
          {{ proxyStore.starting ? '启动中...' : proxyStore.stopping ? '停止中...' : proxyStatus ? '运行中' : '已停止' }}
        </span>
      </div>
    </header>

    <main id="main-content" class="flex-1 overflow-hidden" role="main" aria-label="主要内容区域">
      <Dashboard v-show="uiStore.currentView === 'dashboard'" role="tabpanel" :aria-label="'仪表盘面板'" />
      <ConfigManager v-show="uiStore.currentView === 'configs'" role="tabpanel" :aria-label="'配置管理面板'" />
      <LogViewer v-show="uiStore.currentView === 'logs'" role="tabpanel" :aria-label="'日志查看器面板'" />
      <Doctor v-show="uiStore.currentView === 'doctor'" role="tabpanel" :aria-label="'健康检查面板'" />
      <AuditLog v-show="uiStore.currentView === 'audit'" role="tabpanel" :aria-label="'审计日志面板'" />
      <Settings v-show="uiStore.currentView === 'settings'" role="tabpanel" :aria-label="'系统设置面板'" />
    </main>

    <div 
      class="fixed bottom-4 right-4 flex flex-col gap-2 z-50" 
      role="alert" 
      aria-live="assertive" 
      aria-atomic="true"
      aria-label="通知区域"
    >
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
        role="alert"
        :aria-label="`${notif.type} 通知: ${notif.message}`"
        tabindex="0"
        @keydown.enter="uiStore.dismissNotification(notif.id)"
        @keydown.escape="uiStore.dismissNotification(notif.id)"
      >
        <span class="sr-only">{{ notif.type === 'success' ? '成功' : notif.type === 'error' ? '错误' : notif.type === 'warn' ? '警告' : '信息' }}:</span>
        {{ notif.message }}
        <button 
          class="ml-2 text-current opacity-70 hover:opacity-100" 
          aria-label="关闭通知"
          @click.stop="uiStore.dismissNotification(notif.id)"
        >
          ✕
        </button>
      </div>
    </div>
  </div>
</template>
