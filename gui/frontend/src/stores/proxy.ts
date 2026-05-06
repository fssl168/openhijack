import { defineStore } from 'pinia'
import { StartProxy as StartProxyApi, StopProxy as StopProxyApi, GetStatus as GetStatusApi } from '@/utils/runtime'
import type { StatusInfo as StatusInfoType, LogEntry, LogLevel } from '@/types'

let pollingTimer: ReturnType<typeof setInterval> | null = null

export const useProxyStore = defineStore('proxy', {
  state: () => ({
    running: false as boolean,
    port: 443 as number,
    host: 'localhost' as string,
    currentConfig: null as string | null,
    logs: [] as LogEntry[],
    uptime: '' as string,
    model: '' as string,
    provider: '' as string,
    maxLogs: 500 as number,
  }),

  getters: {
    statusInfo: (state): StatusInfoType => ({
      running: state.running,
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
      try {
        const err = await StartProxyApi(configPath, port)
        if (err) return err
        this.running = true
        this.currentConfig = configPath
        this.port = port
        this.uptime = '刚刚启动'
        return null
      } catch (e: any) {
        return e?.message || '启动失败'
      }
    },

    async stop(): Promise<string | null> {
      try {
        const err = await StopProxyApi()
        if (err) return err
        this.running = false
        this.uptime = ''
        return null
      } catch (e: any) {
        return e?.message || '停止失败'
      }
    },

    async getStatus(): Promise<StatusInfoType | null> {
      try {
        const status = await GetStatusApi() as StatusInfoType
        if (!status) return null
        this.running = status.running
        this.port = status.port
        this.host = status.host
        this.currentConfig = status.config
        this.uptime = status.uptime
        this.model = status.model
        this.provider = status.provider
        return status
      } catch {
        return null
      }
    },

    startPolling(intervalMs: number = 3000) {
      if (pollingTimer) return
      this.getStatus()
      pollingTimer = setInterval(() => {
        this.getStatus()
      }, intervalMs)
    },

    stopPolling() {
      if (pollingTimer) {
        clearInterval(pollingTimer)
        pollingTimer = null
      }
    },
  },
})