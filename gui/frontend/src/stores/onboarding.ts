import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { CreateConfig, SelectConfig, GetSupportedProviders } from '@/utils/runtime'
import type { ConfigData } from '@/types'

export const useOnboardingStore = defineStore('onboarding', {
  state: () => ({
    showOnboarding: false,
    completed: false,
    skipped: false,
    providers: [] as any[],
  }),

  getters: {
    isFirstTime: (state) => !state.completed && !state.skipped,
  },

  actions: {
    async initialize() {
      const hasCompleted = localStorage.getItem('openhijack_onboarding_completed')
      if (!hasCompleted) {
        this.showOnboarding = true
        
        try {
          this.providers = await GetSupportedProviders()
        } catch (error) {
          console.error('Failed to load providers for onboarding:', error)
        }
      }
    },

    async completeOnboarding(data: any) {
      try {
        const configPath = data.path || `~/.config/openhijack/config-${data.provider || 'default'}.toml`

        const config: ConfigData = {
          path: configPath,
          mapped_model_id: data.mapped_model_id || 'default-model',
          auth_key: data.auth_key || this.generateAuthKey(),
          config_groups: [{
            name: 'default',
            provider: data.provider,
            api_url: data.api_url,
            model_id: data.model_id || '',
            api_key: data.api_key,
            middle_route: '/v1',
          }],
        }

        const err = await CreateConfig(config)
        if (err) {
          throw new Error(`创建配置失败: ${err}`)
        }

        await SelectConfig(config.path)

        this.completed = true
        this.showOnboarding = false
        localStorage.setItem('openhijack_onboarding_completed', 'true')

        return true
      } catch (error) {
        console.error('Onboarding completion failed:', error)
        throw error
      }
    },

    skipOnboarding() {
      this.skipped = true
      this.showOnboarding = false
      localStorage.setItem('openhijack_onboarding_skipped', 'true')
    },

    resetOnboarding() {
      this.completed = false
      this.skipped = false
      this.showOnboarding = false
      localStorage.removeItem('openhijack_onboarding_completed')
      localStorage.removeItem('openhijack_onboarding_skipped')
    },

    generateAuthKey(): string {
      const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*'
      let result = ''
      const array = new Uint32Array(32)
      crypto.getRandomValues(array)
      
      for (let i = 0; i < 32; i++) {
        result += chars[array[i] % chars.length]
      }
      
      return result
    },
  },
})
