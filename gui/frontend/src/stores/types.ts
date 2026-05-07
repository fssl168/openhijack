import type { Ref, ComputedRef } from 'vue'

export interface StoreDefinition {
  id: string
  state: Record<string, any>
  getters: Record<string, any>
  actions: Record<string, Function>
}

export interface ProxyStoreState {
  _state: 'stopped' | 'starting' | 'running' | 'stopping'
  port: number
  host: string
  currentConfig: string | null
  logs: Array<{
    timestamp: string
    level: 'info' | 'warn' | 'error'
    message: string
    raw: string
  }>
  uptime: string
  model: string
  provider: string
  maxLogs: number
}

export interface ConfigStoreState {
  configs: Array<{
    path: string
    name: string
    filename: string
    provider?: string
    provider_name?: string
    model_id?: string
    model?: string
    active?: boolean
    is_active?: boolean
  }>
  activeConfig: string | null
  loading: boolean
}

export interface UIStoreState {
  currentView: 'dashboard' | 'configs' | 'logs' | 'settings'
  sidebarCollapsed: boolean
  configEditorOpen: boolean
  editingConfig: any | null
  notifications: Array<{
    id: number
    message: string
    type: 'success' | 'error' | 'info' | 'warn'
    duration: number
  }>
}

export interface OnboardingStoreState {
  showOnboarding: boolean
  completed: boolean
  skipped: boolean
  providers: Array<{
    id: string
    name: string
    default_url: string
    default_route: string
    models: string[]
    api_key_hint: string
  }>
}

export interface AppState {
  _initialized: boolean
  _initializing: boolean
  _error: string | null
}

export type StoreState = ProxyStoreState & ConfigStoreState & UIStoreState & OnboardingStoreState & AppState

export interface StoreGetters {
  proxy: {
    running: ComputedRef<boolean>
    starting: ComputedRef<boolean>
    stopping: ComputedRef<boolean>
    statusInfo: ComputedRef<any>
    filteredLogs: (level?: string) => any[]
  }
  config: {
    activeConfigInfo: ComputedRef<any | undefined>
  }
  ui: {}
  onboarding: {
    isFirstTime: ComputedRef<boolean>
  }
  app: {
    isInitialized: ComputedRef<boolean>
    isInitializing: ComputedRef<boolean>
    hasError: ComputedRef<boolean>
  }
}

export interface StoreActions {
  proxy: {
    start: (configPath: string, port: number) => Promise<string | null>
    stop: () => Promise<string | null>
    addLog: (raw: string) => void
    clearLogs: () => void
    startPolling: (intervalMs?: number) => void
    stopPolling: () => void
    getStatus: () => Promise<void>
    fetchLogs: (limit?: number) => Promise<void>
  }
  config: {
    loadConfigs: () => Promise<string | null>
    createConfig: (data: any) => Promise<string | null>
    updateConfig: (data: any) => Promise<string | null>
    deleteConfig: (path: string) => Promise<string | null>
    testConnection: (configPath: string) => Promise<any | null>
    setActiveConfig: (path: string) => Promise<string | null>
  }
  ui: {
    setView: (view: 'dashboard' | 'configs' | 'logs' | 'settings') => void
    toggleSidebar: () => void
    showNotification: (message: string, type?: 'success' | 'error' | 'info' | 'warn', duration?: number) => void
    dismissNotification: (id: number) => void
  }
  onboarding: {
    initialize: () => Promise<void>
    completeOnboarding: (data: any) => Promise<boolean>
    skipOnboarding: () => void
    resetOnboarding: () => void
  }
  app: {
    initialize: () => Promise<void>
    reset: () => void
  }
}
