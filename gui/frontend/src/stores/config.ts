import { defineStore } from 'pinia'
import { GetConfigs, CreateConfig as CreateConfigApi, UpdateConfig as UpdateConfigApi,
  DeleteConfig as DeleteConfigApi, TestConnection as TestConnectionApi,
  SelectConfig as SelectConfigApi } from '@/utils/runtime'
import type { ConfigInfo, ConfigData, TestResult } from '@/types'

export const useConfigStore = defineStore('config', {
  state: () => ({
    configs: [] as ConfigInfo[],
    activeConfig: null as string | null,
    loading: false as boolean,
  }),

  getters: {
    activeConfigInfo: (state): ConfigInfo | undefined => {
      return state.configs.find((c: ConfigInfo) => c.active)
    },
  },

  actions: {
    async loadConfigs(): Promise<string | null> {
      this.loading = true
      try {
        this.configs = await GetConfigs() || []
        const active = this.configs.find((c: ConfigInfo) => c.active)
        this.activeConfig = active?.path || null
        return null
      } catch (e: any) {
        return e?.message || '加载配置失败'
      } finally {
        this.loading = false
      }
    },

    async createConfig(data: ConfigData): Promise<string | null> {
      try {
        const err = await CreateConfigApi(data as any)
        if (err) return err
        await this.loadConfigs()
        return null
      } catch (e: any) {
        return e?.message || '创建配置失败'
      }
    },

    async updateConfig(data: ConfigData): Promise<string | null> {
      try {
        const err = await UpdateConfigApi(data as any)
        if (err) return err
        await this.loadConfigs()
        return null
      } catch (e: any) {
        return e?.message || '更新配置失败'
      }
    },

    async deleteConfig(path: string): Promise<string | null> {
      try {
        const err = await DeleteConfigApi(path)
        if (err) return err
        await this.loadConfigs()
        return null
      } catch (e: any) {
        return e?.message || '删除配置失败'
      }
    },

    async testConnection(configPath: string): Promise<TestResult | null> {
      try {
        return await TestConnectionApi(configPath) as TestResult
      } catch (e: any) {
        return {
          success: false,
          latency: '',
          message: e?.message || '连接测试失败',
        }
      }
    },

    async setActiveConfig(path: string): Promise<string | null> {
      try {
        await SelectConfigApi(path)
        this.activeConfig = path
        await this.loadConfigs()
        return null
      } catch (e: any) {
        return e?.message || '切换配置失败'
      }
    },
  },
})