<template>
  <div class="space-y-6">
    <div class="text-center">
      <div class="text-6xl mb-4">🎉</div>
      <h3 class="text-2xl font-bold text-white mb-2">配置完成！</h3>
      <p class="text-text-muted max-w-md mx-auto">
        OpenHijack 已准备就绪。以下是你的配置摘要：
      </p>
    </div>

    <!-- 配置摘要 -->
    <div class="bg-bg-primary rounded-lg p-6 border border-border space-y-4">
      <div class="flex items-center justify-between py-3 border-b border-border">
        <span class="text-text-muted">供应商</span>
        <span class="font-medium text-white">{{ providerName }}</span>
      </div>
      
      <div class="flex items-center justify-between py-3 border-b border-border">
        <span class="text-text-muted">API URL</span>
        <span class="font-mono text-sm text-white truncate ml-4 max-w-xs">{{ wizardData.api_url }}</span>
      </div>
      
      <div class="flex items-center justify-between py-3 border-b border-border">
        <span class="text-text-muted">模型 ID</span>
        <span class="font-medium text-white">{{ wizardData.model_id || '默认' }}</span>
      </div>
      
      <div class="flex items-center justify-between py-3 border-b border-border">
        <span class="text-text-muted">API Key</span>
        <span class="font-mono text-sm text-green-400">•••••{{ wizardData.api_key?.slice(-4) }}</span>
      </div>
      
      <div class="flex items-center justify-between py-3">
        <span class="text-text-muted">Auth Key</span>
        <span class="font-mono text-sm text-blue-400">•••••{{ wizardData.auth_key?.slice(-4) }}</span>
      </div>
    </div>

    <!-- 下一步提示 -->
    <div class="bg-primary-light/10 border border-primary-light/30 rounded-lg p-4">
      <h4 class="font-medium text-primary-light mb-2">📌 接下来你可以：</h4>
      <ul class="text-sm text-text-secondary space-y-1 list-disc list-inside">
        <li>在 Dashboard 中启动代理服务</li>
        <li>使用配置的 Auth Key 连接到本地代理</li>
        <li>在设置页面中调整高级选项</li>
        <li>安装 CA 证书以启用 HTTPS 拦截</li>
      </ul>
    </div>

    <!-- 快速开始命令 -->
    <div class="bg-bg-tertiary rounded-lg p-4 font-mono text-xs">
      <p class="text-text-muted mb-2"># 快速开始（在终端中使用）:</p>
      <pre class="text-green-400 overflow-x-auto"><code># 1. 设置环境变量
export OPENHIJACK_AUTH="{{ wizardData.auth_key }}"

# 2. 发送请求到本地代理
curl -H "Authorization: Bearer $OPENHIJACK_AUTH" \
     -H "Content-Type: application/json" \
     https://localhost:443/v1/chat/completions \
     -d '{"model": "{{ wizardData.mapped_model_id || 'default-model' }}", "messages": [{"role": "user", "content": "Hello"}]}'</code></pre>
    </div>

    <!-- 文档链接 -->
    <div class="text-center pt-4">
      <a 
        href="#" 
        @click.prevent="openDocs"
        class="text-primary-light hover:text-primary-light/80 underline text-sm"
      >
        📚 查看完整文档 →
      </a>
    </div>

    <div class="flex justify-end pt-4">
      <button @click="$emit('complete')" class="btn-primary bg-green-600 hover:bg-green-700 px-8 py-3 text-lg">
        🎉 开始使用 OpenHijack
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  wizardData: any
}>()

defineEmits(['complete'])

const providerName = computed(() => {
  const names: Record<string, string> = {
    'openai_chat_completion': 'OpenAI (Chat Completion)',
    'openai_response': 'OpenAI (Responses API)',
    'anthropic': 'Anthropic Claude',
    'gemini': 'Google Gemini',
    'openrouter': 'OpenRouter',
  }
  return names[props.wizardData.provider] || props.wizardData.provider
})

function openDocs() {
  alert('文档功能即将推出！\n\n目前你可以在 GitHub 上查看 README 获取更多信息。')
}
</script>
