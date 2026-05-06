declare module '../../wailsjs/go/main/App' {
  export function CreateConfig(arg1: any): Promise<string>;
  export function DeleteConfig(arg1: string): Promise<string>;
  export function ExportConfig(arg1: string): Promise<string>;
  export function GetConfigs(): Promise<any[]>;
  export function GetLogs(arg1: number): Promise<string[]>;
  export function GetProviderDefaults(arg1: string): Promise<Record<string, string>>;
  export function GetStatus(): Promise<any>;
  export function GetSupportedProviders(): Promise<any[]>;
  export function GetSystemInfo(): Promise<any>;
  export function ImportConfig(arg1: string, arg2: string): Promise<string>;
  export function ImportConfigFromFile(arg1: string, arg2: string): Promise<string>;
  export function ImportConfigFromJSON(arg1: string, arg2: string): Promise<string>;
  export function InstallCACert(): Promise<string>;
  export function LoadConfigFile(arg1: string): Promise<string>;
  export function OpenDirectoryDialog(): Promise<string>;
  export function OpenFileDialog(): Promise<string>;
  export function StartProxy(arg1: string, arg2: number): Promise<string>;
  export function StopProxy(): Promise<string>;
  export function TestConnection(arg1: string): Promise<any>;
  export function UninstallCACert(): Promise<string>;
  export function UpdateConfig(arg1: any): Promise<string>;
}

declare module '../../wailsjs/go/models' {
  export interface ConfigInfo {
    name: string;
    path: string;
    provider: string;
    model: string;
    active: boolean;
  }

  export interface ConfigData {
    path: string;
    mapped_model_id: string;
    auth_key: string;
    config_groups: ConfigGroupData[];
  }

  export interface ConfigGroupData {
    name: string;
    provider: string;
    api_url: string;
    model_id: string;
    api_key: string;
    middle_route: string;
  }

  export interface StatusInfo {
    running: boolean;
    port: number;
    host: string;
    config: string;
    uptime: string;
    model: string;
    provider: string;
  }

  export interface TestResult {
    success: boolean;
    latency: string;
    message: string;
  }

  export interface SystemInfo {
    platform: PlatformInfo;
    go_version: string;
    app_version: string;
  }

  export interface PlatformInfo {
    os: string;
    arch: string;
    privileged: boolean;
    has_sudo: boolean;
    cap_support: boolean;
  }

  export interface ProviderInfo {
    id: string;
    name: string;
    default_url: string;
    default_route: string;
    models: string[];
    api_key_hint: string;
  }
}