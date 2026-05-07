<template>
  <div class="space-y-6">
    <div>
      <h3 class="text-xl font-bold text-white mb-2">配置 API 密钥</h3>
      <p class="text-text-muted">输入你的上游 API 密钥和设置代理认证密钥。</p>
    </div>

    <div class="space-y-4">
      <!-- 上游 API Key -->
      <div class="bg-bg-primary rounded-lg p-4 border border-border">
        <label class="block text-sm font-medium text-text-muted mb-2">
          上游 API Key
          <span class="text-error ml-1">*</span>
        </label>
        
        <input
          v-model="wizardData.api_key"
          type="password"
          :placeholder="apiPlaceholder"
          class="w-full px-4 py-3 bg-bg-secondary border border-border rounded-lg text-white placeholder-text-muted focus:border-primary-light focus:ring-1 focus:ring-primary-light/30 outline-none transition-all"
          autocomplete="off"
        />
        
        <p class="text-xs text-text-muted mt-2">
          这是你的 {{ providerName }} API 密钥，将被用于调用上游服务。
        </p>
      </div>

      <!-- API URL -->
      <div class="bg-bg-primary rounded-lg p-4 border border-border">
        <label class="block text-sm font-medium text-text-muted mb-2">
          API URL
          <span class="text-error ml-1">*</span>
        </label>
        
        <input
          v-model="wizardData.api_url"
          type="url"
          placeholder="https://api.example.com"
          class="w-full px-4 py-3 bg-bg-secondary border border-border rounded-lg text-white placeholder-text-muted focus:border-primary-light focus:ring-1 focus:ring-primary-light/30 outline-none transition-all"
        />
      </div>

      <!-- 代理 Auth Key -->
      <div class="bg-bg-primary rounded-lg p-4 border border-border">
        <label class="block text-sm font-medium text-text-muted mb-2">
          代理认证密钥 (Auth Key)
          <span class="text-error ml-1">*</span>
        </label>
        
        <div class="flex gap-2">
          <input
            v-model="wizardData.auth_key"
            :type="showAuthKey ? 'text' : 'password'"
            placeholder="点击生成或手动输入"
            class="flex-1 px-4 py-3 bg-bg-secondary border border-border rounded-lg text-white placeholder-text-muted focus:border-primary-light focus:ring-1 focus:ring-primary-light/30 outline-none transition-all"
            autocomplete="off"
          />
          
          <button
            @click="wizardData.auth_key = generateKey()"
            class="px-4 py-3 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors whitespace-nowrap"
            title="生成随机密钥"
          >
            🔄 生成
          </button>
          
          <button
            @click="showAuthKey = !showAuthKey"
            class="px-4 py-3 bg-bg-tertiary hover:bg-gray-700 text-white rounded-lg transition-colors"
            :title="showAuthKey ? '隐藏' : '显示'"
          >
            {{ showAuthKey ? '🙈' : '👁️' }}
          </button>
        </div>
        
        <p class="text-xs text-text-muted mt-2">
          客户端连接到 OpenHijack 代理时需要使用此密钥进行认证。
        </p>
      </div>

      <!-- Mapped Model ID -->
      <div class="bg-bg-primary rounded-lg p-4 border border-border">
        <label class="block text-sm font-medium text-text-muted mb-2">
          映射模型 ID (可选)
        </label>
        
        <input
          v-model="wizardData.mapped_model_id"
          type="text"
          placeholder="my-custom-model"
          class="w-full px-4 py-3 bg-bg-secondary border border-border rounded-lg text-white placeholder-text-muted focus:border-primary-light focus:ring-1 focus:ring-primary-light/30 outline-none transition-all"
        />
        
        <p class="text-xs text-text-muted mt-2">
          客户端请求时使用的模型标识符，留空则使用默认值。
        </p>
      </div>
    </div>

    <!-- 安全提示 -->
    <div class="bg-yellow-900/20 border border-yellow-700 rounded-lg p-4">
      <div class="flex items-start gap-3">
        <span class="text-yellow-400 mt-0.5">⚠️</span>
        <div>
          <h4 class="font-medium text-yellow-300 mb-1">安全提示</h4>
          <ul class="text-sm text-yellow-200/80 space-y-1 list-disc list-inside">
            <li>你的 API 密钥将被安全存储在本地配置文件中</li>
            <li>生产环境建议启用加密存储（在设置中开启）</li>
            <li>请勿将密钥分享给他人或提交到版本控制系统</li>
          </ul>
        </div>
      </div>
    </div>

    <div class="flex justify-between pt-4">
      <button @click="$emit('prev')" class="btn-outline">
        ← 返回
      </button>
      <button 
        @click="$emit('next')" 
        :disabled="!isFormValid"
        class="btn-primary"
        :class="{ 'opacity-50 cursor-not-allowed': !isFormValid }"
      >
        下一步 →
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'

const props = defineProps<{
  modelValue: any
}>()

const emit = defineEmits<{
  'update:modelValue': [value: any]
  'next': []
  'prev': []
  'complete': []
}>()

const wizardData = ref(props.modelValue)

watch(() => props.modelValue, (newVal) => {
  wizardData.value = newVal
}, { deep: true })

watch(wizardData, (newVal) => {
  emit('update:modelValue', newVal)
}, { deep: true })

const showAuthKey = ref(false)

const isFormValid = computed(() => {
  return (
    wizardData.value.api_key?.length >= 10 &&
    wizardData.value.api_url !== '' &&
    wizardData.value.auth_key?.length >= 16
  )
})

const providerName = computed(() => {
  const names: Record<string, string> = {
    'openai_chat_completion': 'OpenAI',
    'openai_response': 'OpenAI',
    'anthropic': 'Anthropic',
    'gemini': 'Google Gemini',
    'openrouter': 'OpenRouter',
  }
  return names[wizardData.value.provider] || 'API'
})

const apiPlaceholder = computed(() => {
  const placeholders: Record<string, string> = {
    'openai_chat_completion': 'sk-proj-...',
    'anthropic': 'sk-ant-...',
    'gemini': 'AIza...',
    'openrouter': 'sk-or-...',
  }
  return placeholders[wizardData.value.provider] || 'your-api-key'
})

function generateAuthKey(): string {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*'
  let result = ''
  const array = new Uint32Array(32)
  crypto.getRandomValues(array)

  for (let i = 0; i < 32; i++) {
    result += chars[array[i] % chars.length]
  }

  return result
}

function generateKey() {
  wizardData.value = {
    ...wizardData.value,
    auth_key: generateAuthKey(),
  }
  emit('update:modelValue', wizardData.value)
}
</script>
