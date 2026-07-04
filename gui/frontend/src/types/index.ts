// ============================================
// OpenHijack 前端完整类型定义
// ============================================

// ============================================
// 1. 供应商相关类型 (Provider)
// ============================================

export type ProviderType =
  | 'openai_chat_completion'
  | 'openai_response'
  | 'anthropic'
  | 'gemini'
  | 'openrouter'

export interface Model {
  id: string
  name: string
  context_window?: number
  max_tokens?: number
  pricing?: PricingInfo
}

export interface PricingInfo {
  input_per_token?: number
  output_per_token?: number
  currency?: string
}

export interface ProviderFeature {
  id: string
  name: string
  supported: boolean
}

export interface ProviderInfo {
  id: ProviderType
  name: string
  description?: string
  default_url: string
  default_route: string
  models: Model[]
  api_key_hint: string
  features: ProviderFeature[]
  documentation_url?: string
  icon?: string
}

// ============================================
// 2. 配置相关类型 (Config)
// ============================================

export interface ConfigGroupData {
  name: string
  provider: ProviderType
  api_url: string
  model_id: string
  api_key: string
  middle_route: string
}

export interface ConfigData {
  path: string
  mapped_model_id: string
  auth_key: string
  current_config_index?: number
  config_groups: ConfigGroupData[]
}

export interface ConfigGroup {
  name: string
  provider: ProviderType
  api_url: string
  model_id: string
  middle_route: string
}

export interface ConfigInfo {
  path: string
  filename: string
  name?: string
  provider_name: string
  provider?: ProviderType
  model_id: string
  model?: string
  is_active: boolean
  active?: boolean
  has_errors: boolean
}

// ============================================
// 3. 代理状态类型 (Proxy)
// ============================================

export type ProxyState = 'stopped' | 'starting' | 'running' | 'stopping' | 'error'

export type LogLevel = 'debug' | 'info' | 'warn' | 'error'

export interface StatusInfo {
  running: boolean
  port: number
  host: string
  config: string | null
  uptime: string
  model: string
  provider: string
}

export interface ProxyMeta {
  state: ProxyState
  port: number
  host: string
  current_config: string | null
  pid: number
  uptime_seconds: number
  total_requests: number
  active_connections: number
  error_message?: string
}

export interface LogEntry {
  timestamp: string
  level: 'debug' | 'info' | 'warn' | 'error'
  message: string
  raw: string
  source?: string
  metadata?: Record<string, unknown>
}

// ============================================
// 4. 连接测试类型 (Connection Test)
// ============================================

export interface TestResult {
  success: boolean
  message: string
  latency?: string
  details?: TestDetails
  error_code?: string
}

export interface TestDetails {
  dns_resolve_ms: number
  tcp_connect_ms: number
  tls_handshake_ms: number
  http_request_ms: number
  http_status_code: number
  response_size_bytes: number
}

// ============================================
// 5. 系统信息类型 (System Info)
// ============================================

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
  build_time?: string
  git_commit?: string
}

export interface RuntimeEnv {
  uid: number
  euid: number
  sudo_user: string
  display: string
  xauthority: string
  home: string
  warnings: string[]
}

// ============================================
// 6. UI/通知类型 (UI & Notifications)
// ============================================

export type ViewType = 'dashboard' | 'config' | 'configs' | 'logs' | 'settings' | 'doctor' | 'audit'

export type NotificationType = 'success' | 'error' | 'warning' | 'info'

export interface Notification {
  id: string
  type: NotificationType
  title: string
  message: string
  timestamp: Date
  duration?: number
  action?: {
    label: string
    onClick: () => void
  }
}

// ============================================
// 7. API 响应类型 (API Response)
// ============================================

export interface APIError {
  code: number
  message: string
  user_message?: string
  field?: string
  details?: string
}

export interface APIResponse<T = unknown> {
  success: boolean
  data?: T
  error?: APIError
  meta?: ResponseMeta
}

export interface ResponseMeta {
  request_id: string
  timestamp: string
  version: string
}

// ============================================
// 8. 表单验证类型 (Form Validation)
// ============================================

export interface ValidationRule {
  name: string
  validate: (value: unknown) => string | null
  message?: string
}

export interface FieldError {
  field: string
  message: string
  touched: boolean
}

export interface FormState {
  isValid: boolean
  isDirty: boolean
  isSubmitting: boolean
  errors: Record<string, FieldError>
}

// ============================================
// 9. 导入/导出类型 (Import/Export)
// ============================================

export type ImportFormat = 'toml' | 'json'

export interface ImportOptions {
  format: ImportFormat
  source: 'file' | 'text' | 'clipboard'
  overwrite_existing: boolean
  validate_before_import: boolean
}

export interface ExportOptions {
  format: ImportFormat
  include_secrets: boolean
  destination: 'file' | 'clipboard'
}

// ============================================
// 10. CA 证书类型 (Certificate)
// ============================================

export interface CertificateInfo {
  subject: string
  issuer: string
  serial_number: string
  not_before: Date
  not_after: Date
  is_valid: boolean
  fingerprint_sha256: string
  installed: boolean
  trusted_by_system: boolean
}

// ============================================
// 11. 健康检查 / 审计 / 热重载类型 (Phase B bindings)
// ============================================

export interface DoctorResult {
  name: string
  status: 'PASS' | 'WARN' | 'FAIL'
  detail: string
  fix_hint?: string
}

export interface AuditEntry {
  timestamp: string
  request_id: string
  method: string
  path: string
  status: number
  upstream?: string
  model?: string
  duration_ms: string
  client_ip?: string
  error?: string
}

export interface WatcherStatus {
  running: boolean
  last_reload?: string
  last_error?: string
}
