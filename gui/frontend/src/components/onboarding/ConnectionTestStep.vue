<template>
  <div class="space-y-6">
    <div>
      <h3 class="text-xl font-bold text-white mb-2">测试连接</h3>
      <p class="text-text-muted">验证你的 API 密钥和配置是否正确。</p>
    </div>

    <!-- 测试状态 -->
    <div v-if="!testCompleted" class="bg-bg-primary rounded-lg p-6 border border-border">
      <div class="text-center">
        <div 
          class="w-20 h-20 mx-auto rounded-full flex items-center justify-center text-4xl mb-4"
          :class="testing ? 'bg-blue-900/30 animate-pulse' : 'bg-bg-tertiary'"
        >
          {{ testing ? '⏳' : '🔗' }}
        </div>
        
        <h4 class="font-semibold text-white text-lg mb-2">
          {{ testing ? '正在测试连接...' : '准备就绪' }}
        </h4>
        
        <p class="text-text-muted max-w-md mx-auto mb-6">
          {{ testing 
            ? `正在连接到 ${wizardData.api_url}...` 
            : '点击下方按钮开始测试你的 API 配置是否正常工作'
          }}
        </p>

        <div class="flex justify-center gap-3">
          <button
            @click="runTest"
            :disabled="testing || !canTest"
            class="btn-primary px-8 py-3"
            :class="{ 'opacity-50 cursor-not-allowed': testing || !canTest }"
          >
            {{ testing ? '⏳ 测试中...' : '🚀 开始测试' }}
          </button>
        </div>

        <div v-if="!canTest && !testing" class="mt-4 p-3 bg-yellow-900/20 rounded-lg text-sm text-yellow-200">
          ⚠️ 请先完成 API Key 配置才能进行连接测试
        </div>
      </div>
    </div>

    <!-- 测试结果 -->
    <div v-else class="space-y-4">
      <div 
        class="rounded-lg p-6 border"
        :class="testSuccess 
          ? 'bg-green-900/10 border-green-700' 
          : 'bg-red-900/10 border-red-700'
        "
      >
        <div class="flex items-center gap-4">
          <div 
            class="w-16 h-16 rounded-full flex items-center justify-center text-3xl"
            :class="testSuccess ? 'bg-green-900/30' : 'bg-red-900/30'"
          >
            {{ testSuccess ? '✅' : '❌' }}
          </div>
          
          <div class="flex-1">
            <h4 class="font-semibold text-lg" :class="testSuccess ? 'text-green-400' : 'text-red-400'">
              {{ testSuccess ? '连接成功！' : '连接失败' }}
            </h4>
            
            <p class="text-sm mt-1" :class="testSuccess ? 'text-green-300/80' : 'text-red-300/80'">
              {{ testMessage }}
            </p>
            
            <div v-if="testDetails" class="mt-3 space-y-2">
              <div class="grid grid-cols-2 gap-2 text-xs">
                <div class="bg-bg-primary/50 rounded p-2">
                  <span class="text-text-muted">DNS 解析:</span>
                  <span class="ml-2 text-white">{{ testDetails.dns }}ms</span>
                </div>
                <div class="bg-bg-primary/50 rounded p-2">
                  <span class="text-text-muted">TCP 连接:</span>
                  <span class="ml-2 text-white">{{ testDetails.tcp }}ms</span>
                </div>
                <div class="bg-bg-primary/50 rounded p-2">
                  <span class="text-text-muted">TLS 握手:</span>
                  <span class="ml-2 text-white">{{ testDetails.tls }}ms</span>
                </div>
                <div class="bg-bg-primary/50 rounded p-2">
                  <span class="text-text-muted">HTTP 请求:</span>
                  <span class="ml-2 text-white">{{ testDetails.http }}ms</span>
                </div>
              </div>
              
              <div v-if="testDetails.latency" class="text-sm font-medium mt-2" :class="testSuccess ? 'text-green-400' : 'text-red-400'">
                总延迟: {{ testDetails.latency }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="!testSuccess" class="bg-yellow-900/20 border border-yellow-700 rounded-lg p-4">
        <h4 class="font-medium text-yellow-300 mb-2">💡 故障排除建议</h4>
        <ul class="text-sm text-yellow-200/80 space-y-1 list-disc list-inside">
          <li>检查 API Key 是否正确且未过期</li>
          <li>确认 API URL 格式正确（包含 https://）</li>
          <li>验证网络连接是否正常</li>
          <li>检查是否有防火墙或代理设置阻止连接</li>
        </ul>
        
        <button @click="resetTest" class="mt-3 text-sm underline text-yellow-300 hover:text-yellow-200">
          重新测试
        </button>
      </div>

      <div v-if="testSuccess" class="bg-blue-900/20 border border-blue-700 rounded-lg p-4">
        <h4 class="font-medium text-blue-300 mb-2">✨ 太棒了！</h4>
        <p class="text-sm text-blue-200/80">
          你的配置已通过验证。点击"完成设置"来创建配置文件并启动 OpenHijack。
        </p>
      </div>
    </div>

    <div class="flex justify-between pt-4">
      <button @click="$emit('prev')" class="btn-outline">
        ← 返回
      </button>
      
      <div class="flex gap-3">
        <button 
          v-if="!testCompleted"
          @click="$emit('next')" 
          class="btn-outline"
        >
          跳过测试 →
        </button>
        
        <button 
          v-if="testCompleted"
          @click="$emit('complete')" 
          class="btn-primary bg-green-600 hover:bg-green-700"
        >
          🎉 完成设置
        </button>
        
        <button 
          v-if="!testCompleted"
          @click="$emit('complete')" 
          :disabled="!wizardData.api_key"
          class="btn-primary"
          :class="{ 'opacity-50 cursor-not-allowed': !wizardData.api_key }"
        >
          跳过并完成 →
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { TestConnection, CreateConfig } from '@/utils/runtime'

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

const testing = ref(false)
const testCompleted = ref(false)
const testSuccess = ref(false)
const testMessage = ref('')
const testDetails = ref<any>(null)
const tempConfigPath = ref<string>('')

const canTest = computed(() => {
  return wizardData.value.api_key && wizardData.value.api_url
})

async function runTest() {
  if (!canTest.value) return

  testing.value = true
  testCompleted.value = false

  try {
    const tempPath = `/tmp/openhijack-test-config-${Date.now()}.toml`
    tempConfigPath.value = tempPath

    const createErr = await CreateConfig({
      path: tempPath,
      mapped_model_id: wizardData.value.mapped_model_id || 'test-model',
      auth_key: wizardData.value.auth_key || 'test-auth-key-12345678',
      config_groups: [{
        name: 'test',
        provider: wizardData.value.provider,
        api_url: wizardData.value.api_url,
        model_id: wizardData.value.model_id || '',
        api_key: wizardData.value.api_key,
        middle_route: '/v1',
      }],
    })

    if (createErr) {
      throw new Error(`创建临时配置失败: ${createErr}`)
    }

    const result: any = await TestConnection(tempPath)

    testCompleted.value = true
    testSuccess.value = result.success ?? false

    if (result.success) {
      testMessage.value = result.message || '成功连接到上游 API 服务'

      if (result.details) {
        testDetails.value = {
          dns: Math.round(result.details.dns_resolve_ms || 0),
          tcp: Math.round(result.details.tcp_connect_ms || 0),
          tls: Math.round(result.details.tls_handshake_ms || 0),
          http: Math.round(result.details.http_request_ms || 0),
          latency: result.latency,
        }
      }
    } else {
      testMessage.value = result.message || '无法连接到上游 API 服务'
    }

  } catch (error: any) {
    testCompleted.value = true
    testSuccess.value = false
    testMessage.value = error?.message || error?.user_message || '测试过程中发生未知错误'
  } finally {
    testing.value = false
  }
}

function resetTest() {
  testCompleted.value = false
  testSuccess.value = false
  testMessage.value = ''
  testDetails.value = null
  tempConfigPath.value = ''
}
</script>
