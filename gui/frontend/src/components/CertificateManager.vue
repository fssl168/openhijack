<template>
  <div class="card">
    <div class="flex items-center justify-between mb-4 md:mb-6">
      <h3 class="text-base md:text-lg font-semibold flex items-center gap-2">
        🔐 CA 证书管理
      </h3>
      <span 
        class="status-badge text-xs" 
        :class="certStatus.has_ca ? 'status-running' : 'status-stopped'"
      >
        {{ certStatus.has_ca ? '已生成' : '未生成' }}
      </span>
    </div>

    <!-- 证书状态概览 -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-4 md:mb-6">
      <div class="bg-bg-tertiary rounded-lg p-3">
        <div class="text-xs text-text-muted mb-1">CA 证书</div>
        <div :class="certStatus.has_ca ? 'text-green-400' : 'text-red-400'" class="text-sm font-medium">
          {{ certStatus.has_ca ? '✓ 已存在' : '✗ 不存在' }}
        </div>
      </div>
      <div class="bg-bg-tertiary rounded-lg p-3">
        <div class="text-xs text-text-muted mb-1">服务器证书</div>
        <div :class="certStatus.has_server_cert ? 'text-green-400' : 'text-red-400'" class="text-sm font-medium">
          {{ certStatus.has_server_cert ? '✓ 已存在' : '✗ 不存在' }}
        </div>
      </div>
      <div class="bg-bg-tertiary rounded-lg p-3">
        <div class="text-xs text-text-muted mb-1">系统安装</div>
        <div :class="systemInstalled ? 'text-green-400' : 'text-yellow-400'" class="text-sm font-medium">
          {{ systemInstalled ? '✓ 已信任' : '⚠ 未安装' }}
        </div>
      </div>
      <div class="bg-bg-tertiary rounded-lg p-3">
        <div class="text-xs text-text-muted mb-1">平台</div>
        <div class="text-sm font-medium" :title="`发行版: ${certStatus.distro || '未知'} | CA方法: ${certStatus.ca_method || '未检测到'}`">
          {{ platformIcon }} {{ certStatus.platform_label || certStatus.platform }}
          <span v-if="certStatus.ca_method" class="ml-1.5 text-xs px-1.5 py-0.5 rounded bg-green-900/30 text-green-400 border border-green-700/30">{{ certStatus.ca_method }}</span>
        </div>
      </div>
    </div>

    <!-- 操作按钮组 -->
    <div class="flex flex-wrap gap-2 md:gap-3 mb-4 md:mb-6">
      <button
        @click="handleGenerateAll"
        :disabled="loading"
        class="btn-primary w-full sm:w-auto"
      >
        {{ loading && currentAction === 'generate' ? '⏳ 生成中...' : '🔄 一键生成全部' }}
      </button>

      <button
        @click="handleInstallCA"
        :disabled="loading || !certStatus.has_ca"
        class="btn-success w-full sm:w-auto"
        :class="{ 'opacity-50 cursor-not-allowed': !certStatus.has_ca }"
      >
        {{ loading && currentAction === 'install' ? '⏳ 安装中...' : '📥 安装到系统' }}
      </button>

      <button
        @click="handleUninstallCA"
        :disabled="loading"
        class="btn-error w-full sm:w-auto"
      >
        🗑️ 从系统卸载
      </button>

      <button
        @click="handleRegenerate"
        :disabled="loading"
        class="btn-outline w-full sm:w-auto"
      >
        {{ loading && currentAction === 'regenerate' ? '⏳ 重置中...' : '♻️ 重新生成全部' }}
      </button>
    </div>

    <!-- 操作日志 -->
    <div v-if="logs.length > 0" class="bg-bg-primary rounded-lg p-3 md:p-4 font-mono text-xs max-h-48 overflow-y-auto">
      <div v-for="(log, i) in logs" :key="i" class="py-0.5" :class="log.type === 'error' ? 'text-red-400' : log.type === 'success' ? 'text-green-400' : 'text-text-muted'">
        <span class="text-text-muted mr-2">[{{ log.time }}]</span>{{ log.message }}
      </div>
    </div>

    <!-- 错误/成功消息 -->
    <div v-if="message" :class="[
      'mt-4 p-3 rounded-lg text-xs md:text-sm',
      message.type === 'error' ? 'bg-red-900/30 text-red-300 border border-red-700' :
      message.type === 'success' ? 'bg-green-900/30 text-green-300 border border-green-700' :
      'bg-yellow-900/30 text-yellow-300 border border-yellow-700'
    ]">
      {{ message.text }}
    </div>

    <!-- 详细说明 -->
    <details class="mt-4 text-xs text-text-muted">
      <summary class="cursor-pointer hover:text-text-secondary">📖 证书说明与 TLS 错误排查</summary>
      <div class="mt-3 space-y-2 pl-4">
        <p><strong>什么是 CA 证书？</strong></p>
        <p>OpenHijack 使用自签名 CA 证书来拦截和转发 HTTPS 流量。客户端（浏览器、API 工具）需要信任此 CA 才能正常工作。</p>
        
        <p><strong>为什么会出现 "tls: bad record MAC" 错误？</strong></p>
        <ul class="list-disc list-inside space-y-1 mt-1">
          <li>CA 证书未安装到系统信任库</li>
          <li>客户端使用了缓存的旧证书</li>
          <li>代理服务重启后证书已更换</li>
        </ul>
        
        <p><strong>解决方案：</strong></p>
        <ol class="list-decimal list-inside space-y-1 mt-1">
          <li>点击"一键生成全部"创建新证书</li>
          <li>点击"安装到系统"将 CA 添加到信任库</li>
          <li>如果使用浏览器，可能需要<strong>清除 SSL 状态</strong>或<strong>重启浏览器</strong></li>
          <li>如果使用 API 客户端，设置环境变量：<code class="bg-bg-tertiary px-1 rounded">REQUESTS_CA_BUNDLE=/path/to/ca.crt</code>或<code class="bg-bg-tertiary px-1 rounded">NODE_TLS_REJECT_UNAUTHORIZED=0</code></li>
        </ol>

        <p class="mt-2"><strong>手动安装（命令行）：</strong></p>
        <pre class="bg-bg-tertiary p-2 rounded mt-1 overflow-x-auto"><code># Linux (Debian/Ubuntu/RHEL/Arch) (需要 sudo)
sudo cp {{ certStatus.ca_cert_file || '~/.config/openhijack/ca/ca.crt' }} /usr/local/share/ca-certificates/openhijack-ca.crt
sudo update-ca-certificates

# macOS
sudo security add-trusted-cert -k /Library/Keychains/System.keychain {{ certStatus.ca_cert_file || '~/.config/openhijack/ca/ca.crt' }}

# Windows (管理员 PowerShell)
certutil -addstore ROOT "{{ certStatus.ca_cert_file || '~/.config/openhijack/ca/ca.crt' }}"

# FreeBSD
sudo cp {{ certStatus.ca_cert_file || '~/.config/openhijack/ca/ca.crt' }} /usr/local/share/certs/
sudo certctl rehash</code></pre>
      </div>
    </details>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  GetCertStatus,
  GenerateCACert,
  GenerateServerCerts,
  InstallCACert,
  UninstallCACert,
  RegenerateAllCerts,
} from '@/utils/runtime'

const loading = ref(false)
const currentAction = ref('')
const logs = ref<Array<{ time: string; message: string; type: string }>>([])
const message = ref<{ type: string; text: string } | null>(null)
const certStatus = ref<Record<string, any>>({
  has_ca: false,
  has_server_cert: false,
  ca_dir: '',
  ca_cert_file: '',
  server_cert_file: '',
  platform: '',
})
const systemInstalled = ref(false)

const platformIcon = computed(() => {
  const icons: Record<string, string> = {
    linux: '🐧',
    darwin: '🍎',
    windows: '🪟',
    freebsd: '😈',
  }
  return icons[certStatus.value.platform] || '💻'
})

function addLog(msg: string, type: string = 'info') {
  const now = new Date()
  const time = `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}:${now.getSeconds().toString().padStart(2, '0')}`
  logs.value.push({ time, message: msg, type })
}

function clearMessage() {
  setTimeout(() => { message.value = null }, 5000)
}

async function loadCertStatus() {
  try {
    certStatus.value = await GetCertStatus()
    
    if (certStatus.value.has_ca) {
      addLog(`CA 证书已存在于 ${certStatus.value.ca_dir}`, 'info')
    }
  } catch (e: any) {
    addLog(`获取证书状态失败: ${e?.message || e}`, 'error')
  }
}

async function handleGenerateAll() {
  loading.value = true
  currentAction.value = 'generate'
  message.value = null

  try {
    addLog('开始生成 CA 证书...', 'info')
    const err1 = await GenerateCACert()
    if (err1) throw new Error(err1)
    addLog('✓ CA 证书生成成功', 'success')

    addLog('开始生成服务器证书...', 'info')
    const err2 = await GenerateServerCerts()
    if (err2) throw new Error(err2)
    addLog('✓ 服务器证书生成成功', 'success')

    await loadCertStatus()
    message.value = { type: 'success', text: '所有证书生成完成！建议立即安装到系统。' }
  } catch (e: any) {
    addLog(`生成失败: ${e?.message || e}`, 'error')
    message.value = { type: 'error', text: `生成失败: ${e?.message || e}` }
  } finally {
    loading.value = false
    currentAction.value = ''
    clearMessage()
  }
}

async function handleInstallCA() {
  loading.value = true
  currentAction.value = 'install'
  message.value = null

  try {
    addLog('正在安装 CA 证书到系统...', 'info')
    const err = await InstallCACert()
    if (err) throw new Error(err)
    
    systemInstalled.value = true
    addLog('✓ CA 证书已安装到系统信任库', 'success')
    message.value = { 
      type: 'success', 
      text: 'CA 证书已成功安装！请重启浏览器或 API 客户端以使更改生效。' 
    }
  } catch (e: any) {
    const errMsg = e?.message || e
    addLog(`安装失败: ${errMsg}`, 'error')
    
    if (errMsg.includes('permission') || errMsg.includes('权限') || errMsg.includes('root')) {
      message.value = { 
        type: 'error', 
        text: `需要 root 权限！请运行: sudo openhijack-gui elevate 或手动安装证书` 
      }
    } else {
      message.value = { type: 'error', text: `安装失败: ${errMsg}` }
    }
  } finally {
    loading.value = false
    currentAction.value = ''
    clearMessage()
  }
}

async function handleUninstallCA() {
  loading.value = true
  currentAction.value = 'uninstall'
  message.value = null

  try {
    addLog('正在从系统卸载 CA 证书...', 'info')
    await UninstallCACert()
    
    systemInstalled.value = false
    addLog('✓ CA 证书已从系统移除', 'success')
    message.value = { type: 'success', text: 'CA 证书已从系统卸载。' }
  } catch (e: any) {
    addLog(`卸载过程出错: ${e?.message || e}`, 'error')
    message.value = { type: 'error', text: `卸载失败: ${e?.message || e}` }
  } finally {
    loading.value = false
    currentAction.value = ''
    clearMessage()
  }
}

async function handleRegenerate() {
  if (!confirm('确定要重新生成所有证书吗？这将删除现有证书并创建新的。')) return
  
  loading.value = true
  currentAction.value = 'regenerate'
  message.value = null
  systemInstalled.value = false

  try {
    addLog('正在重置所有证书...', 'info')
    const err = await RegenerateAllCerts()
    if (err) throw new Error(err)
    
    addLog('✓ 所有证书已重新生成', 'success')
    await loadCertStatus()
    message.value = { 
      type: 'success', 
      text: '证书已全部重新生成！建议重新安装到系统并重启浏览器。' 
    }
  } catch (e: any) {
    addLog(`重新生成失败: ${e?.message || e}`, 'error')
    message.value = { type: 'error', text: `重新生成失败: ${e?.message || e}` }
  } finally {
    loading.value = false
    currentAction.value = ''
    clearMessage()
  }
}

onMounted(() => {
  loadCertStatus()
})
</script>
