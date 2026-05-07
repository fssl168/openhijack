function getWailsFunc(name: string): any {
  const go = (window as any).go
  if (!go || !go.main || !go.main.App) {
    throw new Error('Wails runtime 未初始化')
  }
  return go.main.App[name]
}

import type {
  ProviderInfo,
  ConfigData,
  ConfigInfo,
  TestResult,
  SystemInfo,
  RuntimeEnv,
  CertificateInfo,
  StatusInfo,
  ProxyMeta
} from '@/types'

export const CreateConfig = (data: ConfigData): Promise<string | null> => getWailsFunc('CreateConfig')(data)
export const DeleteConfig = (path: string): Promise<string> => getWailsFunc('DeleteConfig')(path)
export const ExportConfig = (path: string): Promise<string> => getWailsFunc('ExportConfig')(path)
export const GetConfigs = (): Promise<ConfigInfo[]> => getWailsFunc('GetConfigs')()
export const GetLogs = (limit: number): Promise<string[]> => getWailsFunc('GetLogs')(limit)
export const GetProviderDefaults = (provider: string): Promise<ProviderInfo | null> => getWailsFunc('GetProviderDefaults')(provider)
export const GetStatus = (): Promise<StatusInfo> => getWailsFunc('GetStatus')()
export const GetSupportedProviders = (): Promise<ProviderInfo[]> => getWailsFunc('GetSupportedProviders')()
export const GetSystemInfo = (): Promise<SystemInfo> => getWailsFunc('GetSystemInfo')()
export const GetRuntimeEnv = (): Promise<RuntimeEnv> => getWailsFunc('GetRuntimeEnv')()
export const ImportConfig = (content: string, path: string): Promise<string> => getWailsFunc('ImportConfig')(content, path)
export const ImportConfigFromFile = (filePath: string, savePath: string): Promise<string> => getWailsFunc('ImportConfigFromFile')(filePath, savePath)
export const ImportConfigFromJSON = (jsonStr: string, savePath: string): Promise<string> => getWailsFunc('ImportConfigFromJSON')(jsonStr, savePath)
export const InstallCACert = (): Promise<string> => getWailsFunc('InstallCACert')()
export const UninstallCACert = (): Promise<string> => getWailsFunc('UninstallCACert')()
export const GetCertStatus = (): Promise<Record<string, any>> => getWailsFunc('GetCertStatus')()
export const GenerateCACert = (): Promise<string> => getWailsFunc('GenerateCACert')()
export const GenerateServerCerts = (): Promise<string> => getWailsFunc('GenerateServerCerts')()
export const RegenerateAllCerts = (): Promise<string> => getWailsFunc('RegenerateAllCerts')()
export const RemoveLocalCerts = (): Promise<string> => getWailsFunc('RemoveLocalCerts')()
export const LoadConfigFile = (path: string): Promise<ConfigData | null> => getWailsFunc('LoadConfigFile')(path)
export const LoadFullConfig = (path: string): Promise<ConfigData> => getWailsFunc('LoadFullConfig')(path)
export const OpenDirectoryDialog = (): Promise<string> => getWailsFunc('OpenDirectoryDialog')()
export const OpenFileDialog = (): Promise<string> => getWailsFunc('OpenFileDialog')()
export const SelectConfig = (path: string): Promise<void> => getWailsFunc('SelectConfig')(path)
export const StartProxy = (configPath: string, port: number): Promise<string> => getWailsFunc('StartProxy')(configPath, port)
export const StopProxy = (): Promise<string> => getWailsFunc('StopProxy')()
export const TestConnection = (path: string): Promise<TestResult> => getWailsFunc('TestConnection')(path)
export const UpdateConfig = (data: ConfigData): Promise<string> => getWailsFunc('UpdateConfig')(data)

// 导出类型（用于向后兼容）
export type { ProxyMeta } from '@/types'

export function isRuntimeReady(): boolean {
  const go = (window as any).go
  return !!(go && go.main && go.main.App)
}