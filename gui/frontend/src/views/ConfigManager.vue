<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useConfigStore } from '@/stores/config'
import { useUIStore } from '@/stores/ui'
import { GetSupportedProviders, ExportConfig, OpenFileDialog, LoadConfigFile,
  ImportConfig, ImportConfigFromFile, ImportConfigFromJSON, isRuntimeReady } from '@/utils/runtime'
import type { ConfigInfo, ConfigData } from '@/types'

const configStore = useConfigStore()
const uiStore = useUIStore()

const showEditor = ref(false)
const editingConfig = ref<ConfigInfo | null>(null)
const testingConnection = ref(false)
const testResult = ref<{ success: boolean; message: string; latency: string } | null>(null)
const showImportModal = ref(false)
const importText = ref('')
const importFilePath = ref('')
const importSavePath = ref('')
const importFormat = ref<'toml' | 'json'>('toml')

const providers = ref<any[]>([])
const formData = ref<ConfigData>({
  path: '',
  mapped_model_id: '',
  auth_key: '',
  config_groups: [{
    name: 'default',
    provider: 'openai_chat_completion',
    api_url: '',
    model_id: '',
    api_key: '',
    middle_route: '/v1',
  }],
})

onMounted(async () => {
  await configStore.loadConfigs()
  loadProviders()
})

async function loadProviders() {
  try {
    providers.value = await GetSupportedProviders() || []
  } catch {
    providers.value = [
      { id: 'openai_chat_completion', name: 'OpenAI 兼容 API', default_url: 'https://api.openai.com', default_route: '/v1', models: ['gpt-4o', 'gpt-4o-mini', 'gpt-4-turbo'], api_key_hint: 'sk-...' },
      { id: 'openrouter', name: 'OpenRouter', default_url: 'https://openrouter.ai/api', default_route: '/v1', models: ['anthropic/claude-sonnet-4', 'openai/gpt-4o'], api_key_hint: 'sk-or-v1-...' },
    ]
  }
}

function handleCreate() {
  formData.value = {
    path: getDefaultConfigPath(),
    mapped_model_id: '',
    auth_key: generateKey(),
    config_groups: [{
      name: 'default',
      provider: 'openai_chat_completion',
      api_url: 'https://api.openai.com',
      model_id: '',
      api_key: '',
      middle_route: '/v1',
    }],
  }
  editingConfig.value = null
  testResult.value = null
  showEditor.value = true
}

function handleEdit(config: ConfigInfo) {
  editingConfig.value = config
  formData.value = {
    path: config.path,
    mapped_model_id: config.name,
    auth_key: '',
    config_groups: [{
      name: config.name,
      provider: config.provider,
      api_url: '',
      model_id: config.model,
      api_key: '',
      middle_route: '/v1',
    }],
  }
  testResult.value = null
  showEditor.value = true
}

async function handleSelectConfig(config: ConfigInfo) {
  if (config.active) return
  const err = await configStore.setActiveConfig(config.path)
  if (err) {
    uiStore.showNotification(`切换配置失败: ${err}`, 'error')
  } else {
    uiStore.showNotification(`已切换到: ${config.name}`, 'success')
  }
}

async function handleSave() {
  if (!formData.value.path) {
    uiStore.showNotification('请指定配置文件路径', 'warn')
    return
  }

  const err = editingConfig.value
    ? await configStore.updateConfig(formData.value)
    : await configStore.createConfig(formData.value)

  if (err) {
    uiStore.showNotification(`保存失败: ${err}`, 'error')
  } else {
    uiStore.showNotification('配置保存成功', 'success')
    showEditor.value = false
  }
}

async function handleDelete(config: ConfigInfo) {
  if (!confirm('确定要删除此配置吗？')) return

  const err = await configStore.deleteConfig(config.path)
  if (err) {
    uiStore.showNotification(`删除失败: ${err}`, 'error')
  } else {
    uiStore.showNotification('配置已删除', 'info')
  }
}

async function handleTestConnection() {
  if (!formData.value.path) {
    uiStore.showNotification('请先保存配置后再测试', 'warn')
    return
  }
  testingConnection.value = true
  testResult.value = null

  const result = await configStore.testConnection(formData.value.path)

  testingConnection.value = false
  if (result) {
    testResult.value = result
  }
}

function generateKey() {
  return Array.from(crypto.getRandomValues(new Uint8Array(16)))
    .map(b => b.toString(16).padStart(2, '0'))
    .join('')
}

function getDefaultConfigPath() {
  return '~/.config/openhijack/config-' + Date.now() + '.toml'
}

const group = computed(() => formData.value.config_groups[0])
const currentProvider = computed(() => providers.value.find(p => p.id === group.value.provider))

watch(() => group.value.provider, (newProvider) => {
  if (newProvider && providers.value.length > 0) {
    const provider = providers.value.find(p => p.id === newProvider)
    if (provider) {
      group.value.api_url = provider.default_url
      group.value.middle_route = provider.default_route
    }
  }
})

async function handleExport(config: ConfigInfo) {
  try {
    const result = await ExportConfig(config.path)
    if (result && result.startsWith('配置')) {
      uiStore.showNotification(result, 'error')
    } else {
      downloadFile(result, `${config.name}-config.toml`, 'text/toml')
      uiStore.showNotification('配置已导出', 'success')
    }
  } catch (e: any) {
    uiStore.showNotification(`导出失败: ${e?.message}`, 'error')
  }
}

function downloadFile(content: string, filename: string, type: string) {
  const blob = new Blob([content], { type })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

async function handleImport() {
  if (!importText.value && !importFilePath.value) {
    uiStore.showNotification('请选择文件或粘贴配置内容', 'warn')
    return
  }

  if (!importSavePath.value) {
    uiStore.showNotification('请指定保存路径', 'warn')
    return
  }

  try {
    let err: string
    if (importFilePath.value) {
      err = await ImportConfigFromFile(importFilePath.value, importSavePath.value)
    } else if (importFormat.value === 'json') {
      err = await ImportConfigFromJSON(importText.value, importSavePath.value)
    } else {
      err = await ImportConfig(importText.value, importSavePath.value)
    }

    if (err) {
      uiStore.showNotification(`导入失败: ${err}`, 'error')
    } else {
      uiStore.showNotification('配置导入成功', 'success')
      showImportModal.value = false
      importText.value = ''
      importFilePath.value = ''
      importSavePath.value = ''
      await configStore.loadConfigs()
    }
  } catch (e: any) {
    uiStore.showNotification(`导入异常: ${e?.message || e}`, 'error')
  }
}

async function handleSelectFile() {
  if (!isRuntimeReady()) {
    uiStore.showNotification('Wails 运行时正在初始化，请稍后再试...', 'warn')
    return
  }

  try {
    const filePath = await OpenFileDialog()
    if (!filePath) return

    importFilePath.value = filePath
    const fileName = filePath.split(/[/\\]/).pop() || filePath
    importSavePath.value = `~/.config/openhijack/${fileName}`

    if (fileName.endsWith('.json')) {
      importFormat.value = 'json'
    } else {
      importFormat.value = 'toml'
    }

    const fileContent = await LoadConfigFile(filePath)
    if (fileContent && !fileContent.startsWith('读取') && !fileContent.startsWith('请')) {
      importText.value = fileContent
    }
  } catch (e: any) {
    uiStore.showNotification(`选择文件失败: ${e?.message || e}`, 'error')
  }
}
</script>

<template>
  <div class="h-full overflow-y-auto p-6">
    <div class="max-w-4xl mx-auto space-y-6">
      <div class="flex items-center justify-between">
        <h2 class="text-xl font-semibold">配置管理</h2>
        <div class="flex gap-3">
          <button @click="showImportModal = true" class="btn-outline">
            📥 导入配置
          </button>
          <button @click="handleCreate" class="btn-primary">
            + 新建配置
          </button>
        </div>
      </div>

      <div v-if="configStore.loading" class="text-center py-12 text-text-muted">
        加载中...
      </div>

      <div v-else-if="configStore.configs.length === 0" class="card text-center py-12">
        <div class="text-4xl mb-4">📝</div>
        <h3 class="text-lg font-semibold mb-2">暂无配置</h3>
        <p class="text-text-muted mb-4">点击「新建配置」或「导入配置」开始</p>
        <button @click="handleCreate" class="btn-primary">
          + 新建配置
        </button>
      </div>

      <div v-else class="space-y-3">
        <div
          v-for="config in configStore.configs"
          :key="config.path"
          class="card flex items-center justify-between cursor-pointer hover:border-primary-light transition-colors"
          :class="{ 'border-primary-light bg-primary-light/5': config.active }"
          @click="handleSelectConfig(config)"
        >
          <div class="flex items-center gap-4">
            <span class="w-2 h-2 rounded-full transition-colors" :class="config.active ? 'bg-green-400 shadow-sm shadow-green-400/50' : 'bg-gray-500'"></span>
            <div>
              <div class="font-medium">{{ config.name }}</div>
              <div class="text-sm text-text-muted">{{ config.provider }} / {{ config.model }}</div>
            </div>
          </div>

          <div class="flex items-center gap-2">
            <span v-if="config.active" class="text-xs text-green-400 mr-2">当前活跃</span>
            <button @click.stop="handleExport(config)" class="btn-outline px-3 py-1 text-sm" title="导出">
              📤
            </button>
            <button @click.stop="handleEdit(config)" class="btn-outline px-3 py-1 text-sm">
              编辑
            </button>
            <button @click.stop="handleDelete(config)" class="btn-outline px-3 py-1 text-sm text-error hover:bg-error/10">
              删除
            </button>
          </div>
        </div>
      </div>

      <div v-if="showEditor" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
        <div class="bg-bg-secondary rounded-xl border border-border w-full max-w-2xl max-h-[90vh] overflow-y-auto">
          <div class="flex items-center justify-between p-6 border-b border-border">
            <h3 class="text-lg font-semibold">
              {{ editingConfig ? '编辑配置' : '新建配置' }}
            </h3>
            <button @click="showEditor = false" class="text-text-muted hover:text-text-primary">
              ✕
            </button>
          </div>

          <div class="p-6 space-y-4">
            <div>
              <label class="block text-sm text-text-muted mb-2">配置文件路径</label>
              <input v-model="formData.path" class="input-field" placeholder="~/.config/openhijack/config.toml" />
            </div>

            <div>
              <label class="block text-sm text-text-muted mb-2">模型 ID (mapped_model_id)</label>
              <input v-model="formData.mapped_model_id" class="input-field" placeholder="my-model" />
            </div>

            <div>
              <label class="block text-sm text-text-muted mb-2">鉴权密钥 (auth_key)</label>
              <div class="flex gap-2">
                <input v-model="formData.auth_key" class="input-field flex-1" placeholder="自动生成" />
                <button @click="formData.auth_key = generateKey()" class="btn-outline">
                  生成
                </button>
              </div>
            </div>

            <div class="pt-4 border-t border-border">
              <h4 class="font-medium mb-4">上游配置 (config_groups[0])</h4>

              <div class="space-y-4">
                <div>
                  <label class="block text-sm text-text-muted mb-2">LLM 供应商</label>
                  <select v-model="group.provider" class="input-field">
                    <option v-for="provider in providers" :key="provider.id" :value="provider.id">
                      {{ provider.name }}
                    </option>
                  </select>
                </div>

                <div v-if="currentProvider" class="bg-bg-primary rounded-lg p-3 text-sm">
                  <div class="text-text-muted mb-2">支持模型:</div>
                  <div class="flex flex-wrap gap-2">
                    <span
                      v-for="model in currentProvider.models"
                      :key="model"
                      @click="group.model_id = model"
                      class="px-2 py-1 bg-bg-tertiary rounded text-xs cursor-pointer hover:bg-primary-light hover:text-white transition-colors"
                      :class="{ 'bg-primary-light text-white': group.model_id === model }"
                    >
                      {{ model }}
                    </span>
                  </div>
                </div>

                <div>
                  <label class="block text-sm text-text-muted mb-2">API URL</label>
                  <input v-model="group.api_url" class="input-field" :placeholder="currentProvider?.default_url" />
                </div>

                <div>
                  <label class="block text-sm text-text-muted mb-2">Model ID</label>
                  <input v-model="group.model_id" class="input-field" :placeholder="currentProvider?.models?.[0] || 'model-name'" />
                </div>

                <div>
                  <label class="block text-sm text-text-muted mb-2">API Key</label>
                  <input v-model="group.api_key" type="password" class="input-field" :placeholder="currentProvider?.api_key_hint || 'api-key'" />
                </div>

                <div>
                  <label class="block text-sm text-text-muted mb-2">中间路由</label>
                  <input v-model="group.middle_route" class="input-field" :placeholder="currentProvider?.default_route || '/v1'" />
                </div>
              </div>
            </div>

            <div v-if="testResult" :class="[
              'p-3 rounded-lg text-sm',
              testResult.success ? 'bg-green-900/30 text-green-300' : 'bg-red-900/30 text-red-300'
            ]">
              {{ testResult.message }}
              <span v-if="testResult.latency">({{ testResult.latency }})</span>
            </div>

            <div class="flex items-center gap-3 pt-4">
              <button @click="handleSave" class="btn-primary">
                保存
              </button>
              <button @click="handleTestConnection" :disabled="testingConnection" class="btn-outline">
                {{ testingConnection ? '测试中...' : '测试连接' }}
              </button>
              <button @click="showEditor = false" class="btn-outline ml-auto">
                取消
              </button>
            </div>
          </div>
        </div>
      </div>

      <div v-if="showImportModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
        <div class="bg-bg-secondary rounded-xl border border-border w-full max-w-2xl max-h-[90vh] overflow-y-auto">
          <div class="flex items-center justify-between p-6 border-b border-border">
            <h3 class="text-lg font-semibold">导入配置</h3>
            <button @click="showImportModal = false" class="text-text-muted hover:text-text-primary">
              ✕
            </button>
          </div>

          <div class="p-6 space-y-4">
            <div>
              <label class="block text-sm text-text-muted mb-2">方式一: 选择配置文件</label>
              <button @click="handleSelectFile" class="btn-outline w-full">
                📂 点击选择文件 (.toml / .json)
              </button>
              <div v-if="importFilePath" class="mt-2 text-sm text-text-secondary break-all">
                已选择: {{ importFilePath }}
              </div>
            </div>

            <div class="text-center text-text-muted">— 或 —</div>

            <div>
              <label class="block text-sm text-text-muted mb-2">方式二: 粘贴配置内容</label>
              <select v-model="importFormat" class="input-field mb-2 w-32">
                <option value="toml">TOML 格式</option>
                <option value="json">JSON 格式</option>
              </select>
              <textarea
                v-model="importText"
                class="input-field font-mono text-xs"
                rows="8"
                placeholder="粘贴 TOML 或 JSON 配置内容..."
              ></textarea>
            </div>

            <div>
              <label class="block text-sm text-text-muted mb-2">保存路径</label>
              <input v-model="importSavePath" class="input-field" placeholder="~/.config/openhijack/imported-config.toml" />
            </div>

            <div class="flex items-center gap-3 pt-4">
              <button @click="handleImport" class="btn-primary">
                导入
              </button>
              <button @click="showImportModal = false" class="btn-outline ml-auto">
                取消
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>