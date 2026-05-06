export interface StatusInfo {
  running: boolean
  port: number
  host: string
  config: string
  uptime: string
  model: string
  provider: string
}

export interface ConfigInfo {
  path: string
  name: string
  provider: string
  model: string
  active: boolean
}

export interface ConfigGroupData {
  name: string
  provider: string
  api_url: string
  model_id: string
  api_key: string
  middle_route: string
}

export interface ConfigData {
  path: string
  mapped_model_id: string
  auth_key: string
  config_groups: ConfigGroupData[]
}

export interface TestResult {
  success: boolean
  latency: string
  message: string
}

export interface PlatformInfo {
  os: string
  arch: string
  privileged: boolean
  has_sudo: boolean
  cap_support: boolean
}

export interface SystemInfo {
  platform: PlatformInfo
  go_version: string
  app_version: string
}

export type LogLevel = 'info' | 'warn' | 'error'

export interface LogEntry {
  timestamp: string
  level: LogLevel
  message: string
  raw: string
}
