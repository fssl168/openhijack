import { defineStore } from 'pinia'
import { StartProxy as StartProxyApi, StopProxy as StopProxyApi, GetStatus as GetStatusApi, GetLogs as GetLogsApi } from '@/utils/runtime'
import type { StatusInfo as StatusInfoType, LogEntry, LogLevel } from '@/types'

type ProxyState = 'stopped' | 'starting' | 'running' | 'stopping'

export const useProxyStore = defineStore('proxy', {
  state: () => ({
    _state: 'stopped' as ProxyState,
    port: 443 as number,
    host: 'localhost' as string,
    currentConfig: null as string | null,
    logs: [] as LogEntry[],
    uptime: '' as string,
    model: '' as string,
    provider: '' as string,
    maxLogs: 500 as number,
    _pollingTimer: null as ReturnType<typeof setInterval> | null,
    _pollingActive: false as boolean,
    _backendConfirmed: false as boolean,
  }),

  getters: {
    running(state): boolean {
      return state._state === 'running' || state._state === 'starting'
    },

    starting(state): boolean {
      return state._state === 'starting'
    },

    stopping(state): boolean {
      return state._state === 'stopping'
    },

    statusInfo: (state): StatusInfoType => ({
      running: state._state === 'running' || state._state === 'starting',
      port: state.port,
      host: state.host,
      config: state.currentConfig || '',
      uptime: state.uptime,
      model: state.model,
      provider: state.provider,
    }),

    filteredLogs: (state) => (level?: LogLevel) => {
      if (!level) return state.logs
      return state.logs.filter(log => log.level === level)
    },
  },

  actions: {
    _transition(newState: ProxyState) {
      const oldState = this._state
      this._state = newState

      if (newState === 'stopped') {
        this._backendConfirmed = false
        this.uptime = ''
      }
      if (newState === 'running') {
        this._backendConfirmed = true
      }
    },

    addLog(raw: string) {
      const entry = this.parseLog(raw)
      if (entry) {
        this.logs.push(entry)
        if (this.logs.length > this.maxLogs) {
          this.logs = this.logs.slice(-this.maxLogs)
        }
      }
    },

    parseLog(raw: string): LogEntry | null {
      const match = raw.match(/\[openhijack\]\s+(\d{2}:\d{2}:\d{2}\.\d{3})\s+\[([^\]]+)\]\s+(.*)/)
      if (match) {
        return {
          timestamp: match[1],
          level: this.detectLogLevel(match[3]),
          message: match[3],
          raw,
        }
      }

      const simpleMatch = raw.match(/\[openhijack\]\s+(\d{2}:\d{2}:\d{2}\.\d{3})\s+(.*)/)
      if (simpleMatch) {
        return {
          timestamp: simpleMatch[1],
          level: this.detectLogLevel(simpleMatch[2]),
          message: simpleMatch[2],
          raw,
        }
      }

      return {
        timestamp: new Date().toLocaleTimeString(),
        level: 'info',
        message: raw,
        raw,
      }
    },

    detectLogLevel(message: string): LogLevel {
      const lower = message.toLowerCase()
      if (lower.includes('error') || lower.includes('fail') || lower.includes('panic')) {
        return 'error'
      }
      if (lower.includes('warn') || lower.includes('deprec') || lower.includes('降级')) {
        return 'warn'
      }
      return 'info'
    },

    clearLogs() {
      this.logs = []
    },

    async start(configPath: string, port: number): Promise<string | null> {
      if (this._state === 'running' || this._state === 'starting') {
        return '代理服务已在运行中'
      }

      this._transition('starting')
      this.currentConfig = configPath
      this.port = port
      this.uptime = '正在启动...'

      try {
        const err = await StartProxyApi(configPath, port)
        if (err) {
          this._transition('stopped')
          return err
        }

        this._transition('running')
        this.uptime = '刚刚启动'

        this._syncFromBackend()
        return null
      } catch (e: any) {
        this._transition('stopped')
        return e?.message || '启动失败'
      }
    },

    async stop(): Promise<string | null> {
      if (this._state === 'stopped' || this._state === 'stopping') {
        return '代理服务未运行'
      }

      this._transition('stopping')

      try {
        const err = await StopProxyApi()
        this._transition('stopped')
        if (err) return err
        return null
      } catch (e: any) {
        this._transition('stopped')
        return e?.message || '停止失败'
      }
    },

    async _syncFromBackend() {
      try {
        const status = await GetStatusApi() as StatusInfoType
        if (!status) return

        this.port = status.port
        this.host = status.host
        this.currentConfig = status.config
        this.uptime = status.uptime
        this.model = status.model
        this.provider = status.provider

        if (status.running && !this.running) {
          this._transition('running')
        } else if (!status.running && this._backendConfirmed && this._state === 'running') {
          this._transition('stopped')
        }

        if (status.running) {
          this._backendConfirmed = true
        }
      } catch {}
    },

    async getStatus() {
      await this._syncFromBackend()
    },

    async fetchLogs(limit: number = 50) {
      try {
        const rawLogs = await GetLogsApi(limit)
        if (rawLogs && Array.isArray(rawLogs)) {
          for (const line of rawLogs) {
            if (typeof line === 'string') {
              this.addLog(line)
            }
          }
        }
      } catch {}
    },

    startPolling(intervalMs: number = 3000) {
      if (this._pollingActive) return
      this._pollingActive = true
      this._syncFromBackend()
      this.fetchLogs()

      this._pollingTimer = setInterval(() => {
        this._syncFromBackend()
        this.fetchLogs()
      }, intervalMs)
    },

    stopPolling() {
      this._pollingActive = false
      if (this._pollingTimer) {
        clearInterval(this._pollingTimer)
        this._pollingTimer = null
      }
    },
  },
})
