import { defineStore } from 'pinia'
import { useProxyStore } from './proxy'
import { useConfigStore } from './config'
import { useUIStore } from './ui'
import { useOnboardingStore } from './onboarding'

export const useAppStore = defineStore('app', {
  state: () => ({
    _initialized: false,
    _initializing: false,
    _error: null as string | null,
  }),

  getters: {
    isInitialized(state): boolean {
      return state._initialized
    },

    isInitializing(state): boolean {
      return state._initializing
    },

    hasError(state): boolean {
      return state._error !== null
    },
  },

  actions: {
    async initialize() {
      if (this._initialized || this._initializing) return

      this._initializing = true
      this._error = null

      try {
        const uiStore = useUIStore()
        const configStore = useConfigStore()
        const onboardingStore = useOnboardingStore()

        await Promise.all([
          configStore.loadConfigs(),
          onboardingStore.initialize(),
        ])

        this._initialized = true
        uiStore.showNotification('应用初始化完成', 'success')
      } catch (e: any) {
        this._error = e?.message || '初始化失败'
        
        const uiStore = useUIStore()
        uiStore.showNotification(`初始化失败: ${this._error}`, 'error')
      } finally {
        this._initializing = false
      }
    },

    reset() {
      this._initialized = false
      this._initializing = false
      this._error = null

      const proxyStore = useProxyStore()
      const configStore = useConfigStore()
      const uiStore = useUIStore()
      const onboardingStore = useOnboardingStore()

      proxyStore.stopPolling()
      onboardingStore.resetOnboarding()
    },
  },
})
