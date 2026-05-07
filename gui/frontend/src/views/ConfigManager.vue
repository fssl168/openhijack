<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useConfigStore } from '@/stores/config'
import { useUIStore } from '@/stores/ui'
import { GetSupportedProviders, ExportConfig, OpenFileDialog, LoadConfigFile,
  ImportConfig, ImportConfigFromFile, ImportConfigFromJSON, isRuntimeReady, LoadFullConfig } from '@/utils/runtime'
import type { ConfigInfo, ConfigData, TestResult } from '@/types'
import FormField from '@/components/FormField.vue'
import { useFormValidation } from '@/composables/useFormValidation'
import {
  required, minLength, maxLength, url as urlValidator, flexibleUrl
} from '@/utils/validation'

const configStore = useConfigStore()
const uiStore = useUIStore()

const showEditor = ref(false)
const editingConfig = ref<ConfigInfo | null>(null)
const testingConnection = ref(false)
const testResult = ref<TestResult | null>(null)
const submitting = ref(false)
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

// 表单验证 schema
const validationSchema = {
  path: [required('请指定配置文件路径')],
  mapped_model_id: [required('请输入模型 ID'), maxLength(100)],
  auth_key: [minLength(8, '鉴权密钥如果填写，至少需要 8 个字符，建议 16 位以上更安全')],
  'config_groups[0].provider': [required('请选择 LLM 供应商')],
  'config_groups[0].api_url': [required('请输入 API URL'), flexibleUrl('请输入有效的域名或完整 URL（如 api.openai.com 或 http(s)://api.openai.com）')],
  'config_groups[0].api_key': [required('请输入上游 API Key'), minLength(6, 'API Key 至少需要 6 个字符')],
}

// 使用表单验证 composable（带 500ms 防抖）
const {
  errors,
  touched,
  isValid,
  isDirty,
  validateField,
  validateAll,
  touchField,
  resetForm,
  getFieldError,
} = useFormValidation(formData.value, validationSchema, { debounceDelay: 500 })

onMounted(() => {
  if (isRuntimeReady()) {
    configStore.loadConfigs()
    loadProviders()
  } else {
    setTimeout(() => {
      configStore.loadConfigs()
      loadProviders()
    }, 500)
  }
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

async function handleEdit(config: ConfigInfo) {
  editingConfig.value = config
  testResult.value = null
  showEditor.value = true

  try {
    const fullConfig = await LoadFullConfig(config.path)

    formData.value = {
      path: fullConfig.path || config.path,
      mapped_model_id: fullConfig.mapped_model_id || config.name || config.model_id || '',
      auth_key: fullConfig.auth_key || '',
      config_groups: fullConfig.config_groups && fullConfig.config_groups.length > 0
        ? fullConfig.config_groups.map(g => ({
            name: g.name || config.name || 'default',
            provider: g.provider || config.provider || 'openai_chat_completion',
            api_url: g.api_url || '',
            model_id: g.model_id || config.model || config.model_id || '',
            api_key: g.api_key || '',
            middle_route: g.middle_route || '/v1',
          }))
        : [{
            name: config.name || 'default',
            provider: config.provider || 'openai_chat_completion',
            api_url: '',
            model_id: config.model || config.model_id || '',
            api_key: '',
            middle_route: '/v1',
          }],
    }
  } catch (error) {
    console.error('加载完整配置失败:', error)
    uiStore.showNotification('加载配置详情失败，请手动填写', 'warn')

    formData.value = {
      path: config.path,
      mapped_model_id: config.name || config.model_id || '',
      auth_key: '',
      config_groups: [{
        name: config.name || 'default',
        provider: config.provider || 'openai_chat_completion',
        api_url: '',
        model_id: config.model || config.model_id || '',
        api_key: '',
        middle_route: '/v1',
      }],
    }
  }
}

async function handleSelectConfig(config: ConfigInfo) {
  if (config.is_active || config.active) return
  const err = await configStore.setActiveConfig(config.path)
  if (err) {
    uiStore.showNotification(`切换配置失败: ${err}`, 'error')
  } else {
    uiStore.showNotification(`已切换到: ${config.name || config.filename}`, 'success')
  }
}

async function handleSave() {
  // 先进行表单验证
  const isFormValid = validateAll(formData.value as unknown as Record<string, any>)
  
  if (!isFormValid) {
    uiStore.showNotification('请检查表单中的错误', 'error')
    return
  }

  submitting.value = true
  
  try {
    const err = editingConfig.value
      ? await configStore.updateConfig(formData.value)
      : await configStore.createConfig(formData.value)

    if (err) {
      uiStore.showNotification(`保存失败: ${err}`, 'error')
    } else {
      uiStore.showNotification('配置保存成功', 'success')
      showEditor.value = false
      resetForm()
    }
  } finally {
    submitting.value = false
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
    if (fileContent) {
      // 将 ConfigData 转换为 TOML 字符串显示
      importText.value = JSON.stringify(fileContent, null, 2)
    }
  } catch (e: any) {
    uiStore.showNotification(`选择文件失败: ${e?.message || e}`, 'error')
  }
}
</script>

<template>
  <div class="h-full overflow-y-auto p-4 md:p-6">
    <div class="max-w-4xl mx-auto space-y-4 md:space-y-6">
      <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <h2 class="text-lg md:text-xl font-semibold">配置管理</h2>
        <div class="flex flex-col sm:flex-row gap-2 sm:gap-3 w-full sm:w-auto">
          <button @click="showImportModal = true" class="btn-outline w-full sm:w-auto">
            📥 导入配置
          </button>
          <button @click="handleCreate" class="btn-primary w-full sm:w-auto">
            + 新建配置
          </button>
        </div>
      </div>

      <div v-if="configStore.loading" class="text-center py-8 md:py-12 text-text-muted">
        加载中...
      </div>

      <div v-else-if="configStore.configs.length === 0" class="card text-center py-8 md:py-12">
        <div class="text-3xl md:text-4xl mb-3 md:mb-4">📝</div>
        <h3 class="text-base md:text-lg font-semibold mb-2">暂无配置</h3>
        <p class="text-sm md:text-base text-text-muted mb-3 md:mb-4">点击「新建配置」或「导入配置」开始</p>
        <button @click="handleCreate" class="btn-primary w-full sm:w-auto max-w-xs mx-auto block">
          + 新建配置
        </button>
      </div>

      <div v-else class="space-y-2 md:space-y-3">
        <div
          v-for="config in configStore.configs"
          :key="config.path"
          class="card flex flex-col sm:flex-row items-start sm:items-center justify-between cursor-pointer hover:border-primary-light transition-colors gap-3"
          :class="{ 'border-primary-light bg-primary-light/5': config.is_active || config.active }"
          @click="handleSelectConfig(config)"
        >
          <div class="flex items-center gap-3 flex-1 min-w-0">
            <span class="w-2 h-2 rounded-full flex-shrink-0 transition-colors" :class="(config.is_active || config.active) ? 'bg-green-400 shadow-sm shadow-green-400/50' : 'bg-gray-500'"></span>
            <div class="min-w-0 flex-1">
              <div class="font-medium text-sm md:text-base truncate">{{ config.name || config.filename }}</div>
              <div class="text-xs md:text-sm text-text-muted truncate">{{ config.provider_name }}{{ config.provider ? ` / ${config.provider}` : '' }}{{ config.model_id || config.model ? ` / ${config.model || config.model_id}` : '' }}</div>
            </div>
          </div>

          <div class="flex flex-wrap items-center gap-2 w-full sm:w-auto justify-end">
            <span v-if="config.is_active || config.active" class="text-xs text-green-400 mr-1">当前活跃</span>
            <button @click.stop="handleExport(config)" class="btn-outline px-2 md:px-3 py-1 text-xs md:text-sm" title="导出">
              📤
            </button>
            <button @click.stop="handleEdit(config)" class="btn-outline px-2 md:px-3 py-1 text-xs md:text-sm">
              编辑
            </button>
            <button @click.stop="handleDelete(config)" class="btn-outline px-2 md:px-3 py-1 text-xs md:text-sm text-error hover:bg-error/10">
              删除
            </button>
          </div>
        </div>
      </div>

      <div v-if="showEditor" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
        <div class="bg-bg-secondary rounded-xl border border-border w-full max-w-2xl max-h-[95vh] md:max-h-[90vh] overflow-y-auto">
          <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between p-4 md:p-6 border-b border-border gap-2">
            <h3 class="text-base md:text-lg font-semibold">
              {{ editingConfig ? '编辑配置' : '新建配置' }}
            </h3>
            <button @click="showEditor = false" class="text-text-muted hover:text-text-primary self-end sm:self-auto">
              ✕
            </button>
          </div>

          <div class="p-4 md:p-6 space-y-3 md:space-y-4">
            <!-- 配置文件路径 -->
            <FormField
              label="配置文件路径"
              :error="getFieldError('path')"
              :touched="touched['path']"
              :required="true"
              help="配置文件将保存到 ~/.config/openhijack/ 目录"
              :value="formData.path"
              @blur="touchField('path')"
            >
              <input
                v-model="formData.path"
                class="input-field"
                placeholder="~/.config/openhijack/config.toml"
                @blur="touchField('path')"
              />
            </FormField>

            <!-- 模型 ID -->
            <FormField
              label="模型 ID (mapped_model_id)"
              :error="getFieldError('mapped_model_id')"
              :touched="touched['mapped_model_id']"
              :required="true"
              help="客户端连接时使用的模型标识符"
              :value="formData.mapped_model_id"
              @blur="touchField('mapped_model_id')"
            >
              <input
                v-model="formData.mapped_model_id"
                class="input-field"
                placeholder="my-model"
                @blur="touchField('mapped_model_id')"
              />
            </FormField>

            <!-- 鉴权密钥 -->
            <FormField
              label="鉴权密钥 (auth_key)"
              :error="getFieldError('auth_key')"
              :touched="touched['auth_key']"
              :required="true"
              help="用于客户端认证的密钥（至少 16 位）"
              :value="formData.auth_key"
              @blur="touchField('auth_key')"
            >
              <div class="flex flex-col sm:flex-row gap-2">
                <input
                  v-model="formData.auth_key"
                  type="password"
                  class="input-field flex-1"
                  placeholder="点击生成按钮自动创建"
                  @blur="touchField('auth_key')"
                />
                <button
                  type="button"
                  @click="formData.auth_key = generateKey(); touchField('auth_key')"
                  class="btn-outline whitespace-nowrap w-full sm:w-auto"
                >
                  🔄 生成
                </button>
              </div>
            </FormField>

            <div class="pt-3 md:pt-4 border-t border-border">
              <h4 class="font-medium mb-3 md:mb-4 text-sm md:text-base">上游配置 (config_groups[0])</h4>

              <div class="space-y-3 md:space-y-4">
                <!-- LLM 供应商 -->
                <FormField
                  label="LLM 供应商"
                  :error="getFieldError('config_groups[0].provider')"
                  :touched="touched['config_groups[0].provider']"
                  :required="true"
                  help="选择 API 提供商"
                  :value="group.provider"
                  @blur="touchField('config_groups[0].provider')"
                >
                  <select
                    v-model="group.provider"
                    class="input-field"
                    @blur="touchField('config_groups[0].provider')"
                  >
                    <option value="">请选择供应商</option>
                    <option v-for="provider in providers" :key="provider.id" :value="provider.id">
                      {{ provider.name }}
                    </option>
                  </select>
                </FormField>

                <!-- 模型选择器 -->
                <div v-if="currentProvider" class="bg-bg-primary rounded-lg p-2 md:p-3 text-xs md:text-sm">
                  <div class="text-text-muted mb-2">支持模型 (点击选择):</div>
                  <div class="flex flex-wrap gap-1 md:gap-2">
                    <span
                      v-for="model in currentProvider.models"
                      :key="model"
                      @click="group.model_id = model"
                      class="px-2 py-1 bg-bg-tertiary rounded text-xs cursor-pointer hover:bg-primary-light hover:text-white transition-colors"
                      :class="{ 'bg-primary-light text-white': group.model_id === model }"
                      tabindex="0"
                      role="option"
                      :aria-selected="group.model_id === model"
                    >
                      {{ model }}
                    </span>
                  </div>
                </div>

                <!-- API URL -->
                <FormField
                  label="API URL"
                  :error="getFieldError('config_groups[0].api_url')"
                  :touched="touched['config_groups[0].api_url']"
                  :required="true"
                  help="上游 API 的完整 URL 地址"
                  :value="group.api_url"
                  @blur="touchField('config_groups[0].api_url')"
                >
                  <input
                    v-model="group.api_url"
                    class="input-field"
                    type="url"
                    :placeholder="currentProvider?.default_url || 'https://api.example.com'"
                    @blur="touchField('config_groups[0].api_url')"
                  />
                </FormField>

                <!-- Model ID -->
                <FormField
                  label="Model ID"
                  :error="getFieldError('config_groups[0].model_id')"
                  :touched="touched['config_groups[0].model_id']"
                  help="要调用的具体模型名称"
                  :value="group.model_id"
                  @blur="touchField('config_groups[0].model_id')"
                >
                  <input
                    v-model="group.model_id"
                    class="input-field"
                    :placeholder="currentProvider?.models?.[0] || 'gpt-4o'"
                    @blur="touchField('config_groups[0].model_id')"
                  />
                </FormField>

                <!-- API Key -->
                <FormField
                  label="上游 API Key"
                  :error="getFieldError('config_groups[0].api_key')"
                  :touched="touched['config_groups[0].api_key']"
                  :required="true"
                  help="用于认证上游 API 的密钥（至少 10 位）"
                  :value="group.api_key"
                  @blur="touchField('config_groups[0].api_key')"
                >
                  <input
                    v-model="group.api_key"
                    type="password"
                    class="input-field"
                    :placeholder="currentProvider?.api_key_hint || 'sk-...'"
                    autocomplete="off"
                    @blur="touchField('config_groups[0].api_key')"
                  />
                </FormField>

                <!-- 中间路由 -->
                <FormField
                  label="中间路由 (Middle Route)"
                  help="API 路径前缀，通常为 /v1"
                  :value="group.middle_route"
                  @blur="touchField('config_groups[0].middle_route')"
                >
                  <input
                    v-model="group.middle_route"
                    class="input-field"
                    :placeholder="currentProvider?.default_route || '/v1'"
                    @blur="touchField('config_groups[0].middle_route')"
                  />
                </FormField>
              </div>
            </div>

            <div v-if="testResult" :class="[
              'p-3 rounded-lg text-xs md:text-sm',
              testResult.success ? 'bg-green-900/30 text-green-300' : 'bg-red-900/30 text-red-300'
            ]">
              {{ testResult.message }}
              <span v-if="testResult.latency">({{ testResult.latency }})</span>
            </div>

            <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-2 sm:gap-3 pt-3 md:pt-4">
              <button
                type="button"
                @click="handleSave"
                :disabled="submitting || !isValid"
                :class="['btn-primary w-full sm:w-auto', { 'opacity-50 cursor-not-allowed': submitting || !isValid }]"
              >
                {{ submitting ? '⏳ 保存中...' : '✓ 保存配置' }}
              </button>
              <button
                type="button"
                @click="handleTestConnection"
                :disabled="testingConnection || !formData.path"
                class="btn-outline w-full sm:w-auto"
              >
                {{ testingConnection ? '🔄 测试中...' : '🔗 测试连接' }}
              </button>
              <button
                type="button"
                @click="showEditor = false; resetForm()"
                class="btn-outline w-full sm:w-auto ml-0 sm:ml-auto"
              >
                取消
              </button>
            </div>
          </div>
        </div>
      </div>

      <div v-if="showImportModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
        <div class="bg-bg-secondary rounded-xl border border-border w-full max-w-2xl max-h-[95vh] md:max-h-[90vh] overflow-y-auto">
          <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between p-4 md:p-6 border-b border-border gap-2">
            <h3 class="text-base md:text-lg font-semibold">导入配置</h3>
            <button @click="showImportModal = false" class="text-text-muted hover:text-text-primary self-end sm:self-auto">
              ✕
            </button>
          </div>

          <div class="p-4 md:p-6 space-y-3 md:space-y-4">
            <div>
              <label class="block text-xs md:text-sm text-text-muted mb-2">方式一: 选择配置文件</label>
              <button @click="handleSelectFile" class="btn-outline w-full py-3 md:py-2">
                📂 点击选择文件 (.toml / .json)
              </button>
              <div v-if="importFilePath" class="mt-2 text-xs md:text-sm text-text-secondary break-all">
                已选择: {{ importFilePath }}
              </div>
            </div>

            <div class="text-center text-text-muted text-sm">— 或 —</div>

            <div>
              <label class="block text-xs md:text-sm text-text-muted mb-2">方式二: 粘贴配置内容</label>
              <select v-model="importFormat" class="input-field mb-2 w-28 md:w-32">
                <option value="toml">TOML 格式</option>
                <option value="json">JSON 格式</option>
              </select>
              <textarea
                v-model="importText"
                class="input-field font-mono text-xs"
                rows="6 md:rows-8"
                placeholder="粘贴 TOML 或 JSON 配置内容..."
              ></textarea>
            </div>

            <div>
              <label class="block text-xs md:text-sm text-text-muted mb-2">保存路径</label>
              <input v-model="importSavePath" class="input-field" placeholder="~/.config/openhijack/imported-config.toml" />
            </div>

            <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-2 sm:gap-3 pt-3 md:pt-4">
              <button @click="handleImport" class="btn-primary w-full sm:w-auto">
                导入
              </button>
              <button @click="showImportModal = false" class="btn-outline w-full sm:w-auto ml-0 sm:ml-auto">
                取消
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>