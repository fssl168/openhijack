<template>
  <div class="space-y-6">
    <div>
      <h3 class="text-xl font-bold text-white mb-2">选择 LLM 供应商</h3>
      <p class="text-text-muted">选择你要使用的 AI 模型供应商，我们将自动配置相关设置。</p>
    </div>

    <div class="grid grid-cols-1 gap-3">
      <button
        v-for="provider in providers"
        :key="provider.id"
        @click="selectProvider(provider)"
        class="p-4 rounded-lg border transition-all text-left"
        :class="[
          wizardData.provider === provider.id
            ? 'border-primary-light bg-primary-light/10 ring-2 ring-primary-light/30'
            : 'border-border hover:border-primary-light/50 bg-bg-primary hover:bg-bg-tertiary'
        ]"
      >
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div
              class="w-10 h-10 rounded-lg flex items-center justify-center text-xl"
              :class="getProviderBgClass(provider.id)"
            >
              {{ getProviderIcon(provider.id) }}
            </div>
            <div>
              <h4 class="font-semibold text-white">{{ provider.name }}</h4>
              <p class="text-sm text-text-muted">{{ provider.description || provider.default_url }}</p>
            </div>
          </div>

          <div
            v-if="wizardData.provider === provider.id"
            class="w-6 h-6 rounded-full bg-primary-light flex items-center justify-center"
          >
            ✓
          </div>
        </div>

        <div v-if="wizardData.provider === provider.id && provider.models?.length" class="mt-3 pt-3 border-t border-border">
          <p class="text-xs text-text-muted mb-2">支持的模型:</p>
          <div class="flex flex-wrap gap-2">
            <span
              v-for="model in provider.models.slice(0, 8)"
              :key="model"
              @click.stop="selectModel(model)"
              class="px-2 py-1 text-xs rounded cursor-pointer transition-all"
              :class="[
                wizardData.model_id === model
                  ? 'bg-primary-light text-white'
                  : 'bg-bg-tertiary text-text-muted hover:text-white'
              ]"
            >
              {{ model }}
            </span>
          </div>
        </div>
      </button>
    </div>

    <div v-if="providers.length === 0" class="text-center py-8 text-text-muted">
      正在加载供应商列表...
    </div>

    <div class="flex justify-between pt-4">
      <button @click="$emit('prev')" class="btn-outline">
        ← 返回
      </button>
      <button 
        @click="$emit('next')" 
        :disabled="!wizardData.provider"
        class="btn-primary"
        :class="{ 'opacity-50 cursor-not-allowed': !wizardData.provider }"
      >
        下一步 →
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { GetSupportedProviders } from '@/utils/runtime'

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

const providers = ref<any[]>([])

onMounted(async () => {
  try {
    providers.value = await GetSupportedProviders()

    if (wizardData.value.provider && providers.value.length > 0) {
      const selected = providers.value.find(p => p.id === wizardData.value.provider)
      if (selected) {
        wizardData.value.api_url = selected.default_url || ''
        emit('update:modelValue', wizardData.value)
      }
    }
  } catch (error) {
    console.error('Failed to load providers:', error)
  }
})

function selectProvider(provider: any) {
  wizardData.value = {
    ...wizardData.value,
    provider: provider.id,
    api_url: provider.default_url || '',
  }

  if (provider.models?.length && !wizardData.value.model_id) {
    wizardData.value.model_id = provider.models[0]
  }

  emit('update:modelValue', wizardData.value)
}

function selectModel(model: string) {
  wizardData.value = {
    ...wizardData.value,
    model_id: model,
  }
  emit('update:modelValue', wizardData.value)
}

function getProviderIcon(id: string): string {
  const icons: Record<string, string> = {
    'openai_chat_completion': '🤖',
    'openai_response': '🤖',
    'anthropic': '🧠',
    'gemini': '✨',
    'openrouter': '🌐',
  }
  return icons[id] || '📦'
}

function getProviderBgClass(id: string): string {
  const classes: Record<string, string> = {
    'openai_chat_completion': 'bg-green-900/30',
    'openai_response': 'bg-green-900/30',
    'anthropic': 'bg-orange-900/30',
    'gemini': 'bg-blue-900/30',
    'openrouter': 'bg-purple-900/30',
  }
  return classes[id] || 'bg-gray-800/30'
}
</script>
