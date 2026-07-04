import {
  CreateConfig,
  DeleteConfig,
  ExportConfig,
  GetConfigs,
  GetLogs,
  GetProviderDefaults,
  GetStatus,
  GetSupportedProviders,
  GetSystemInfo,
  GetRuntimeEnv,
  ImportConfig,
  ImportConfigFromFile,
  ImportConfigFromJSON,
  InstallCACert,
  LoadConfigFile,
  OpenFileDialog,
  SelectConfig,
  StartProxy,
  StopProxy,
  TestConnection,
  UninstallCACert,
  UpdateConfig,
  isRuntimeReady,
  RunDoctor,
  GetLastDoctorResults,
  GetDoctorSummary,
  GetAuditLogs,
  GetAuditLogPath,
  ClearAuditLogs,
  GetWatcherStatus,
  ReloadConfigManually,
  onConfigReloaded,
} from '../utils/runtime'
import type {
  ConfigData,
  ConfigInfo,
  TestResult,
  SystemInfo,
  RuntimeEnv,
  StatusInfo,
  ProviderInfo,
  DoctorResult,
  AuditEntry,
  WatcherStatus,
} from '@/types'

export class ConfigService {
  static async loadAll(): Promise<ConfigInfo[]> {
    if (!isRuntimeReady()) {
      throw new Error('Wails 运行时未初始化')
    }
    
    const configs = await GetConfigs()
    return configs || []
  }

  static async create(data: ConfigData): Promise<string | null> {
    this.validateConfigData(data)
    return await CreateConfig(data)
  }

  static async update(data: ConfigData): Promise<string | null> {
    this.validateConfigData(data)
    return await UpdateConfig(data)
  }

  static async delete(path: string): Promise<string | null> {
    if (!path) {
      throw new Error('配置路径不能为空')
    }
    return await DeleteConfig(path)
  }

  static async exportToFile(config: ConfigInfo): Promise<{ success: boolean; content?: string; error?: string }> {
    try {
      const result = await ExportConfig(config.path)
      
      if (result && result.startsWith('配置')) {
        return { success: false, error: result }
      }
      
      return { success: true, content: result }
    } catch (e: any) {
      return { success: false, error: e?.message || '导出失败' }
    }
  }

  static async importFromContent(content: string, savePath: string): Promise<string | null> {
    if (!content || !savePath) {
      throw new Error('配置内容和保存路径不能为空')
    }
    return await ImportConfig(content, savePath)
  }

  static async importFromFile(filePath: string, savePath: string): Promise<string | null> {
    if (!filePath || !savePath) {
      throw new Error('文件路径和保存路径不能为空')
    }
    return await ImportConfigFromFile(filePath, savePath)
  }

  static async importFromJSON(jsonStr: string, savePath: string): Promise<string | null> {
    try {
      JSON.parse(jsonStr)
    } catch {
      throw new Error('无效的 JSON 格式')
    }
    return await ImportConfigFromJSON(jsonStr, savePath)
  }

  static async selectActive(path: string): Promise<void> {
    if (!path) {
      throw new Error('配置路径不能为空')
    }
    await SelectConfig(path)
  }

  static async testConnection(configPath: string): Promise<TestResult> {
    if (!configPath) {
      throw new Error('配置路径不能为空')
    }
    
    const result = await TestConnection(configPath)
    return result || {
      success: false,
      latency: '',
      message: '连接测试失败',
    }
  }

  static async loadFileForImport(): Promise<{ filePath: string; content?: ConfigData; format: 'toml' | 'json' }> {
    if (!isRuntimeReady()) {
      throw new Error('Wails 运行时正在初始化，请稍后再试...')
    }

    const filePath = await OpenFileDialog()
    if (!filePath) {
      throw new Error('未选择文件')
    }

    let format: 'toml' | 'json' = 'toml'
    if (filePath.endsWith('.json')) {
      format = 'json'
    }

    const content = await LoadConfigFile(filePath)
    
    return { filePath, content: content || undefined, format }
  }

  private static validateConfigData(data: ConfigData): void {
    if (!data.path) {
      throw new Error('配置文件路径不能为空')
    }
    if (!data.mapped_model_id) {
      throw new Error('模型 ID 不能为空')
    }
    if (!data.auth_key || data.auth_key.length < 16) {
      throw new Error('鉴权密钥至少需要 16 个字符')
    }
    if (!data.config_groups || data.config_groups.length === 0) {
      throw new Error('至少需要一个配置组')
    }
    
    const group = data.config_groups[0]
    if (!group.provider) {
      throw new Error('LLM 供应商不能为空')
    }
    if (!group.api_url) {
      throw new Error('API URL 不能为空')
    }
    if (!group.api_key || group.api_key.length < 10) {
      throw new Error('API Key 至少需要 10 个字符')
    }
  }
}

export class ProxyService {
  static async start(configPath: string, port: number): Promise<{ success: boolean; error?: string }> {
    if (!configPath) {
      return { success: false, error: '配置路径不能为空' }
    }
    if (port < 1 || port > 65535) {
      return { success: false, error: '端口号必须在 1-65535 范围内' }
    }

    const error = await StartProxy(configPath, port)
    
    if (error) {
      return { success: false, error }
    }
    
    return { success: true }
  }

  static async stop(): Promise<{ success: boolean; error?: string }> {
    const error = await StopProxy()
    
    if (error) {
      return { success: false, error }
    }
    
    return { success: true }
  }

  static async getStatus(): Promise<StatusInfo | null> {
    try {
      return await GetStatus()
    } catch {
      return null
    }
  }

  static async fetchLogs(limit: number = 50): Promise<string[]> {
    try {
      const logs = await GetLogs(limit)
      return logs || []
    } catch {
      return []
    }
  }
}

export class ProviderService {
  static async getAll(): Promise<ProviderInfo[]> {
    try {
      const providers = await GetSupportedProviders()
      return providers || this.getDefaultProviders()
    } catch {
      return this.getDefaultProviders()
    }
  }

  static async getDefaults(providerId: string): Promise<ProviderInfo | null> {
    try {
      return await GetProviderDefaults(providerId)
    } catch {
      return null
    }
  }

  private static getDefaultProviders(): ProviderInfo[] {
    return [
      {
        id: 'openai_chat_completion',
        name: 'OpenAI 兼容 API',
        default_url: 'https://api.openai.com',
        default_route: '/v1',
        models: [
          { id: 'gpt-4o', name: 'GPT-4o' },
          { id: 'gpt-4o-mini', name: 'GPT-4o Mini' },
          { id: 'gpt-4-turbo', name: 'GPT-4 Turbo' },
        ],
        api_key_hint: 'sk-...',
        features: [],
      },
      {
        id: 'openrouter',
        name: 'OpenRouter',
        default_url: 'https://openrouter.ai/api',
        default_route: '/v1',
        models: [
          { id: 'anthropic/claude-sonnet-4', name: 'Claude Sonnet 4' },
          { id: 'openai/gpt-4o', name: 'GPT-4o (via OpenRouter)' },
        ],
        api_key_hint: 'sk-or-v1-...',
        features: [],
      },
    ]
  }
}

export class SystemService {
  static async getInfo(): Promise<SystemInfo> {
    try {
      const info = await GetSystemInfo()
      return info || this.getDefaultInfo()
    } catch {
      return this.getDefaultInfo()
    }
  }

  static async getRuntimeEnv(): Promise<RuntimeEnv> {
    try {
      const env = await GetRuntimeEnv()
      return env || this.getDefaultEnv()
    } catch {
      return this.getDefaultEnv()
    }
  }

  static async installCACert(): Promise<{ success: boolean; error?: string }> {
    try {
      const error = await InstallCACert()
      
      if (error) {
        return { success: false, error }
      }
      
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || '安装失败' }
    }
  }

  static async uninstallCACert(): Promise<{ success: boolean; error?: string }> {
    try {
      await UninstallCACert()
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || '卸载失败' }
    }
  }

  private static getDefaultInfo(): SystemInfo {
    return {
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

  private static getDefaultEnv(): RuntimeEnv {
    return {
      uid: 0,
      euid: 0,
      sudo_user: '',
      display: '',
      xauthority: '',
      home: '',
      warnings: [],
    }
  }
}

export class DoctorService {
  static async runChecks(): Promise<DoctorResult[]> {
    try {
      const results = await RunDoctor()
      return results || []
    } catch {
      return []
    }
  }

  static async getLastResults(): Promise<DoctorResult[]> {
    try {
      const results = await GetLastDoctorResults()
      return results || []
    } catch {
      return []
    }
  }

  static async getSummary(): Promise<{ pass: number; warn: number; fail: number }> {
    try {
      const summary = await GetDoctorSummary()
      return {
        pass: summary?.pass || 0,
        warn: summary?.warn || 0,
        fail: summary?.fail || 0,
      }
    } catch {
      return { pass: 0, warn: 0, fail: 0 }
    }
  }
}

export class AuditService {
  static async getLogs(limit = 100, offset = 0): Promise<AuditEntry[]> {
    try {
      const entries = await GetAuditLogs(limit, offset)
      return entries || []
    } catch {
      return []
    }
  }

  static async getLogPath(): Promise<string> {
    try {
      return (await GetAuditLogPath()) || ''
    } catch {
      return ''
    }
  }

  static async clearLogs(): Promise<{ success: boolean; error?: string }> {
    try {
      const msg = await ClearAuditLogs()
      if (msg) {
        return { success: false, error: msg }
      }
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || '清空审计日志失败' }
    }
  }
}

export class WatcherService {
  static async getStatus(): Promise<WatcherStatus | null> {
    try {
      return await GetWatcherStatus()
    } catch {
      return null
    }
  }

  static async reloadManually(): Promise<{ success: boolean; error?: string }> {
    try {
      const msg = await ReloadConfigManually()
      if (msg) {
        return { success: false, error: msg }
      }
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || '重载配置失败' }
    }
  }

  static onConfigReloaded(handler: (payload: WatcherStatus) => void): void {
    onConfigReloaded(handler)
  }
}

export const Services = {
  config: ConfigService,
  proxy: ProxyService,
  provider: ProviderService,
  system: SystemService,
  doctor: DoctorService,
  audit: AuditService,
  watcher: WatcherService,
}
