<template>
  <div class="space-y-6">
    <div class="text-center">
      <div class="text-6xl mb-4">🚀</div>
      <h3 class="text-xl font-bold text-white mb-2">欢迎使用 OpenHijack</h3>
      <p class="text-text-muted max-w-md mx-auto">
        OpenHijack 是一个本地 HTTPS 代理服务器，可以帮助你安全地管理和转发 API 请求。
        让我们用几分钟时间完成初始配置。
      </p>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mt-8">
      <div class="bg-bg-primary rounded-lg p-4 border border-border">
        <div class="flex items-center gap-3">
          <span class="text-2xl">🔒</span>
          <div>
            <h4 class="font-semibold text-white">安全加密</h4>
            <p class="text-sm text-text-muted">TLS 端到端加密保护你的 API 密钥</p>
          </div>
        </div>
      </div>

      <div class="bg-bg-primary rounded-lg p-4 border border-border">
        <div class="flex items-center gap-3">
          <span class="text-2xl">⚡</span>
          <div>
            <h4 class="font-semibold text-white">高性能代理</h4>
            <p class="text-sm text-text-muted">本地代理，低延迟，高吞吐量</p>
          </div>
        </div>
      </div>

      <div class="bg-bg-primary rounded-lg p-4 border border-border">
        <div class="flex items-center gap-3">
          <span class="text-2xl">🎯</span>
          <div>
            <h4 class="font-semibold text-white">多供应商支持</h4>
            <p class="text-sm text-text-muted">支持 OpenAI、Anthropic、Gemini 等</p>
          </div>
        </div>
      </div>

      <div class="bg-bg-primary rounded-lg p-4 border border-border">
        <div class="flex items-center gap-3">
          <span class="text-2xl">🛠️</span>
          <div>
            <h4 class="font-semibold text-white">易于配置</h4>
            <p class="text-sm text-text-muted">图形化界面，一键启动和管理</p>
          </div>
        </div>
      </div>
    </div>

    <div class="bg-blue-900/20 border border-blue-700 rounded-lg p-4 mt-6">
      <div class="flex items-start gap-3">
        <span class="text-blue-400 mt-0.5">💡</span>
        <div>
          <h4 class="font-medium text-blue-300 mb-1">提示</h4>
          <p class="text-sm text-blue-200/80">
            你需要准备一个上游 API 的密钥（如 OpenAI API Key）。
            如果还没有，可以先跳过向导，稍后在设置页面中配置。
          </p>
        </div>
      </div>
    </div>

    <div class="flex justify-end pt-4">
      <button @click="$emit('next')" class="btn-primary">
        开始配置 →
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
defineEmits(['next', 'prev', 'complete'])
defineModel<any>('wizardData')
</script>
