import { defineStore } from 'pinia'
import { StartProxy as StartProxyApi, StopProxy as StopProxyApi, GetStatus as GetStatusApi, GetLogs as GetLogsApi } from '@/utils/runtime'
import type { StatusInfo, LogEntry, LogLevel } from '@/types'

type ProxyState = 'stopped' | 'starting' | 'running' | 'stopping'

// 轮询配置
const POLLING_CONFIG = {
  baseInterval: 3000,        // 基础轮询间隔（毫秒）
  minInterval: 1000,         // 最小间隔
  maxInterval: 30000,        // 最大间隔（30秒）
  backoffFactor: 1.5,        // 退避倍数
  activeMultiplier: 0.5,     // 运行状态时的加速因子（更频繁）
  initialDelay: 100,         // 初始延迟
  maxConsecutiveNoChange: 3, // 连续无变化次数后开始退避
}

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
    _crashDetected: false as boolean,
    _consecutiveDownCount: 0 as number,
    
    // 新增：智能轮询状态
    _currentInterval: POLLING_CONFIG.baseInterval,
    _consecutiveNoChangeCount: 0,
    _lastStatusHash: '' as string,
    _lastLogCount: 0,
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

    statusInfo: (state): StatusInfo => ({
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
      this._state = newState

      if (newState === 'stopped') {
        this.uptime = ''
        this._crashDetected = false
        this._consecutiveDownCount = 0
      }
      if (newState === 'running') {
        this._crashDetected = false
        this._consecutiveDownCount = 0
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
        return { timestamp: match[1], level: this.detectLogLevel(match[3]), message: match[3], raw }
      }

      const simpleMatch = raw.match(/\[openhijack\]\s+(\d{2}:\d{2}:\d{2}\.\d{3})\s+(.*)/)
      if (simpleMatch) {
        return { timestamp: simpleMatch[1], level: this.detectLogLevel(simpleMatch[2]), message: simpleMatch[2], raw }
      }

      return { timestamp: new Date().toLocaleTimeString(), level: 'info', message: raw, raw }
    },

    detectLogLevel(message: string): LogLevel {
      const lower = message.toLowerCase()
      if (lower.includes('error') || lower.includes('fail') || lower.includes('panic')) return 'error'
      if (lower.includes('warn') || lower.includes('deprec') || lower.includes('降级')) return 'warn'
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
        this._syncMetaFromBackend()

        return null
      } catch (e: any) {
        this._transition('stopped')
        return e?.message || '启动失败'
      }
    },

    async stop(): Promise<string | null> {
      if (this._state === 'stopping') {
        return '正在停止中...'
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

    async _syncMetaFromBackend() {
      try {
        const status = await GetStatusApi() as StatusInfo
        if (!status) return

        this.port = status.port
        this.host = status.host
        if (status.config) this.currentConfig = status.config
        this.uptime = status.uptime || this.uptime
        this.model = status.model
        this.provider = status.provider

        // 更新状态哈希（用于变化检测）
        this._lastStatusHash = this._computeStatusHash()
        this._lastLogCount = this.logs.length

        this._checkCrash(status)
      } catch {}
    },

    _checkCrash(status: StatusInfo) {
      if (this._state !== 'running' && this._state !== 'starting') return

      if (!status.running) {
        this._consecutiveDownCount++

        if (this._consecutiveDownCount >= 5) {
          this._crashDetected = true
          this._transition('stopped')
          this.uptime = ''
        }
      } else {
        this._consecutiveDownCount = 0
      }
    },

    async getStatus() {
      await this._syncMetaFromBackend()
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

    startPolling(intervalMs: number = POLLING_CONFIG.baseInterval) {
      if (this._pollingActive) return
      this._pollingActive = true
      this._currentInterval = intervalMs
      this._consecutiveNoChangeCount = 0

      // 初始延迟后立即执行一次
      setTimeout(() => {
        this._performPollingCycle()
      }, POLLING_CONFIG.initialDelay)

      // 启动智能轮询循环
      this._startSmartPolling()
    },

    _startSmartPolling() {
      if (this._pollingTimer) {
        clearInterval(this._pollingTimer)
      }

      this._pollingTimer = setInterval(() => {
        this._performPollingCycle()
      }, this._currentInterval)
    },

    async _performPollingCycle() {
      if (!this._pollingActive) return

      const prevStatusHash = this._lastStatusHash
      const prevLogCount = this._lastLogCount

      try {
        // 执行数据获取
        await this._syncMetaFromBackend()
        await this.fetchLogs()

        // 检测是否有变化
        const hasStatusChanged = this._lastStatusHash !== prevStatusHash
        const hasNewLogs = this.logs.length > prevLogCount
        const hasAnyChange = hasStatusChanged || hasNewLogs

        // 根据变化调整轮询间隔
        this._adjustPollingInterval(hasAnyChange)

      } catch (error) {
        console.warn('[ProxyStore] Polling cycle error:', error)
        
        // 错误时增加间隔（退避）
        this._increaseInterval()
      }
    },

    _adjustPollingInterval(hasChange: boolean) {
      if (hasChange) {
        // 有变化：重置计数器，加速轮询
        this._consecutiveNoChangeCount = 0
        
        // 如果服务正在运行，使用更快的频率
        if (this.running) {
          this._currentInterval = Math.max(
            POLLING_CONFIG.minInterval,
            Math.floor(POLLING_CONFIG.baseInterval * POLLING_CONFIG.activeMultiplier)
          )
        } else {
          this._currentInterval = POLLING_CONFIG.baseInterval
        }
      } else {
        // 无变化：增加计数器
        this._consecutiveNoChangeCount++
        
        if (this._consecutiveNoChangeCount >= POLLING_CONFIG.maxConsecutiveNoChange) {
          // 达到阈值，开始指数退避
          this._increaseInterval()
        }
      }

      // 如果间隔发生变化，重启定时器
      this._restartPollingIfNeeded()
    },

    _increaseInterval() {
      const newInterval = Math.floor(
        this._currentInterval * POLLING_CONFIG.backoffFactor
      )
      
      this._currentInterval = Math.min(
        newInterval,
        POLLING_CONFIG.maxInterval
      )
      
      console.debug(`[ProxyStore] Backoff: interval=${this._currentInterval}ms`)
    },

    _resetInterval() {
      this._currentInterval = POLLING_CONFIG.baseInterval
      this._consecutiveNoChangeCount = 0
    },

    _computeStatusHash(): string {
      return JSON.stringify({
        state: this._state,
        port: this.port,
        uptime: this.uptime,
        model: this.model,
        provider: this.provider,
      })
    },

    _restartPollingIfNeeded() {
      // 仅当间隔变化较大时才重启（避免频繁重启）
      const currentTimerInterval = this._pollingTimer ? 
        // 无法直接获取 setInterval 的间隔，所以通过比较判断
        this._currentInterval : 
        0
      
      if (Math.abs(currentTimerInterval - this._currentInterval) > 500) {
        this._startSmartPolling()
      }
    },

    stopPolling() {
      this._pollingActive = false
      this._resetInterval()
      
      if (this._pollingTimer) {
        clearInterval(this._pollingTimer)
        this._pollingTimer = null
      }
    },
  },
})
