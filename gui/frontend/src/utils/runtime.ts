function getWailsFunc(name: string): any {
  const go = (window as any).go
  if (!go || !go.main || !go.main.App) {
    throw new Error('Wails runtime 未初始化')
  }
  return go.main.App[name]
}

export const CreateConfig = (arg1: any) => getWailsFunc('CreateConfig')(arg1)
export const DeleteConfig = (arg1: string) => getWailsFunc('DeleteConfig')(arg1)
export const ExportConfig = (arg1: string) => getWailsFunc('ExportConfig')(arg1)
export const GetConfigs = () => getWailsFunc('GetConfigs')()
export const GetLogs = (arg1: number) => getWailsFunc('GetLogs')(arg1)
export const GetProviderDefaults = (arg1: string) => getWailsFunc('GetProviderDefaults')(arg1)
export const GetStatus = () => getWailsFunc('GetStatus')()
export const GetSupportedProviders = () => getWailsFunc('GetSupportedProviders')()
export const GetSystemInfo = () => getWailsFunc('GetSystemInfo')()
export const ImportConfig = (arg1: string, arg2: string) => getWailsFunc('ImportConfig')(arg1, arg2)
export const ImportConfigFromFile = (arg1: string, arg2: string) => getWailsFunc('ImportConfigFromFile')(arg1, arg2)
export const ImportConfigFromJSON = (arg1: string, arg2: string) => getWailsFunc('ImportConfigFromJSON')(arg1, arg2)
export const InstallCACert = () => getWailsFunc('InstallCACert')()
export const LoadConfigFile = (arg1: string) => getWailsFunc('LoadConfigFile')(arg1)
export const OpenDirectoryDialog = () => getWailsFunc('OpenDirectoryDialog')()
export const OpenFileDialog = () => getWailsFunc('OpenFileDialog')()
export const StartProxy = (arg1: string, arg2: number) => getWailsFunc('StartProxy')(arg1, arg2)
export const StopProxy = () => getWailsFunc('StopProxy')()
export const TestConnection = (arg1: string) => getWailsFunc('TestConnection')(arg1)
export const UninstallCACert = () => getWailsFunc('UninstallCACert')()
export const UpdateConfig = (arg1: any) => getWailsFunc('UpdateConfig')(arg1)

export function isRuntimeReady(): boolean {
  const go = (window as any).go
  return !!(go && go.main && go.main.App)
}