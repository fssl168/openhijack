# OpenHijack 项目架构与 UI 交互设计审查报告

**项目名称**: OpenHijack - 本地 HTTPS 代理服务器  
**审查日期**: 2026-05-07  
**审查范围**: 架构设计、代码质量、UI/UX 设计、安全性、可维护性  
**审查版本**: v1.0.0

---

## 📋 目录

1. [执行摘要](#1-执行摘要)
2. [架构设计问题](#2-架构设计问题)
3. [UI/UX 交互设计问题](#3uiux-交互设计问题)
4. [安全性问题](#4安全性问题)
5. [性能与可扩展性问题](#5性能与可扩展性问题)
6. [代码质量问题](#6代码质量问题)
7. [改进建议优先级矩阵](#7改进建议优先级矩阵)
8. [实施路线图](#8实施路线图)

---

## 1. 执行摘要

### 🎯 总体评价

**评分**: ⭐⭐⭐☆☆ (3/5)  
**成熟度**: **Beta 阶段** - 核心功能可用，但存在多处需要优化的设计和实现问题

### ✅ 优点
- ✅ 清晰的模块化目录结构（CLI/GUI 分离）
- ✅ 使用现代技术栈（Go + Vue3 + Wails + Pinia）
- ✅ 跨平台支持（Linux/Windows/macOS 抽象层）
- ✅ 完整的配置管理功能
- ✅ TLS 证书自动管理

### ❌ 主要问题
- ❌ **架构层面**: 缺少依赖注入、错误处理不一致、状态管理混乱
- ❌ **UI 层面**: 表单验证缺失、用户体验不佳、响应式设计不足
- ❌ **安全层面**: API Key 明文存储、认证机制简单
- ❌ **性能层面**: 轮询机制低效、内存管理待优化

---

## 2. 架构设计问题

### 🔴 问题 2.1: 前后端职责划分不清

**严重程度**: 🔴 高  
**位置**: [gui/app.go](gui/app.go), [gui/frontend/src/stores/config.ts](gui/frontend/src/stores/config.ts)

#### 现状分析

```go
// gui/app.go - 后端承担了过多业务逻辑
func (a *App) CreateConfig(data ConfigData) string {
    data.Path = a.resolveConfigPath(data.Path)  // 路径解析
    dir := filepath.Dir(data.Path)
    if err := mkdirAll(dir); err != nil {        // 目录创建
        return fmt.Sprintf("创建配置目录失败: %v", err)
    }
    content := a.buildConfigContent(data)         // 内容构建
    if err := os.WriteFile(data.Path, []byte(content), 0600); err != nil {
        return fmt.Sprintf("写入配置文件失败: %v", err)
    }
    return ""
}
```

#### 问题说明

1. **业务逻辑泄漏**: GUI 层（`app.go`）包含了本应属于 `config` 包的配置构建逻辑
2. **违反单一职责**: `App` 结构体既负责 Wails 绑定，又负责文件操作、路径解析、内容序列化
3. **难以测试**: 业务逻辑与框架耦合，无法独立单元测试

#### 改进建议

```go
// 推荐方案：引入 Service 层
type ConfigService interface {
    Create(ctx context.Context, data ConfigData) error
    Update(ctx context.Context, data ConfigData) error
    Delete(ctx context.Context, path string) error
    List(ctx context.Context) ([]ConfigInfo, error)
}

type configService struct {
    configRepo   ConfigRepository
    pathResolver PathResolver
    validator    ConfigValidator
}
```

---

### 🔴 问题 2.2: 错误处理策略不统一

**严重程度**: 🔴 高  
**位置**: 全局

#### 现状分析

```go
// 方式 1: 返回字符串错误
func (a *App) CreateConfig(data ConfigData) string {
    return fmt.Sprintf("创建配置目录失败: %v", err)
}

// 方式 2: 返回 error 类型
func (a *App) StartProxy(configPath string, port int) string {
    return fmt.Sprintf("端口 %d 绑定失败: %v", port, err)
}

// 方式 3: 直接 panic/exit
if _, err := os.Stat(configPath); os.IsNotExist(err) {
    fmt.Fprintf(os.Stderr, "配置文件不存在: %s\n", ...)
    os.Exit(1)
}
```

#### 问题说明

1. **错误类型混乱**: 混用 `string`、`error`、`os.Exit()`
2. **错误信息不规范**: 中英文混杂，格式不统一
3. **缺少错误分类**: 未区分用户错误、系统错误、网络错误
4. **前端处理困难**: 字符串错误难以程序化处理

#### 改进建议

```go
// 定义统一错误类型
type AppError struct {
    Code       ErrorCode `json:"code"`
    Message    string    `json:"message"`
    UserMsg    string    `json:"user_message"` // 用户友好的消息
    Details    string    `json:"details,omitempty"`
    InnerError error     `json:"-"`
}

type ErrorCode int

const (
    ErrInvalidInput ErrorCode = iota + 1001
    ErrNotFound
    ErrPermissionDenied
    ErrNetwork
    ErrInternal
)

// 统一返回格式
type APIResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   *AppError   `json:"error,omitempty"`
}
```

---

### 🟡 问题 2.3: 状态管理过于分散

**严重程度**: 🟡 中  
**位置**: [gui/frontend/src/stores/](gui/frontend/src/stores/)

#### 现状分析

```typescript
// stores/proxy.ts - 代理状态
export const useProxyStore = defineStore('proxy', {
  state: () => ({
    _state: 'stopped' as ProxyState,
    port: 443,
    currentConfig: null as string | null,
    // ... 更多状态
  })
})

// stores/config.ts - 配置状态
export const useConfigStore = defineStore('config', {
  state: () => ({
    configs: [] as ConfigInfo[],
    activeConfig: null as string | null,
  })
})

// stores/ui.ts - UI 状态
export const useUIStore = defineStore('ui', {
  state: () => ({
    currentView: 'dashboard',
    notifications: [],
  })
})
```

#### 问题说明

1. **状态边界模糊**: `activeConfig` 同时存在于 `config.ts` 和 `proxy.ts`
2. **缺乏全局状态协调**: 多个 Store 之间通过组件手动同步
3. **无持久化机制**: 刷新页面后状态丢失
4. **调试困难**: 无法追踪状态变更历史

#### 改进建议

```typescript
// 方案 A: 引入组合式 Store
export const useAppStore = defineStore('app', () => {
  const proxy = useProxyStore()
  const config = useConfigStore()
  
  // 协调逻辑
  async function startProxyWithConfig() {
    if (!config.activeConfig) {
      ui.showNotification('请先选择配置', 'warn')
      return
    }
    await proxy.start(config.activeConfig, proxy.port)
  }
  
  return { startProxyWithConfig }
})

// 方案 B: 使用插件自动持久化
// pinia-plugin-persistedstate
```

---

### 🟡 问题 2.4: 配置系统设计缺陷

**严重程度**: 🟡 中  
**位置**: [internal/config/config.go](internal/config/config.go), [gui/app.go](gui/app.go)

#### 现状分析

```go
type Config struct {
    MappedModelID       string        `toml:"mapped_model_id"`
    AuthKey             string        `toml:"auth_key"`
    CurrentConfigIndex  int           `toml:"current_config_index"` // 数组索引！
    ConfigGroups        []ConfigGroup `toml:"config_groups"`
}
```

#### 问题说明

1. **使用数组索引引用当前配置**: `CurrentConfigIndex` 是脆弱的设计，删除元素会导致索引错乱
2. **配置 ID 缺失**: 没有唯一标识符，难以在多个配置间切换
3. **TOML 格式限制**: 复杂嵌套结构可读性差
4. **无版本控制**: 无法检测配置格式变更

#### 改进建议

```go
type Config struct {
    ID              string            `toml:"id" json:"id"` // UUID
    Version         int               `toml:"version" json:"version"`
    MappedModelID   string            `toml:"mapped_model_id"`
    AuthKey         string            `toml:"auth_key"`
    ActiveGroupID   string            `toml:"active_group_id"` // 用 ID 替代索引
    ConfigGroups    []ConfigGroup     `toml:"config_groups"`
    CreatedAt       time.Time         `toml:"created_at"`
    UpdatedAt       time.Time         `toml:"updated_at"`
    Metadata        map[string]string `toml:"metadata"` // 扩展字段
}
```

---

## 3. UI/UX 交互设计问题

### 🔴 问题 3.1: 表单验证完全缺失

**严重程度**: 🔴 高  
**位置**: [views/ConfigManager.vue](gui/frontend/src/views/ConfigManager.vue#L298-L420)

#### 现状分析

```vue
<template>
  <div class="p-6 space-y-4">
    <div>
      <label>配置文件路径</label>
      <!-- 无验证规则 -->
      <input v-model="formData.path" placeholder="~/.config/openhijack/config.toml" />
    </div>
    
    <div>
      <label>API URL</label>
      <!-- 无 URL 格式校验 -->
      <input v-model="group.api_url" placeholder="https://api.openai.com" />
    </div>
    
    <div>
      <label>API Key</label>
      <!-- 无必填检查 -->
      <input v-model="group.api_key" type="password" placeholder="api-key" />
    </div>
    
    <button @click="handleSave">保存</button> <!-- 直接调用，无前置验证 -->
  </div>
</template>
```

#### 问题说明

1. **无实时验证**: 用户提交后才可能发现错误
2. **无格式校验**: URL、Email、API Key 格式未验证
3. **无必填项提示**: 必填字段无视觉标识
4. **错误反馈差**: 只显示 Toast，无法定位具体字段

#### 改进建议

```vue
<template>
  <form @submit.prevent="handleSubmit">
    <FormField 
      v-model="formData.path"
      label="配置文件路径"
      :rules="[required, validPath]"
      :error="errors.path"
    />
    
    <FormField
      v-model="group.api_url"
      label="API URL"
      :rules="[required, url]"
      :error="errors.api_url"
      placeholder="https://api.openai.com"
    />
    
    <button 
      type="submit"
      :disabled="!isFormValid || submitting"
    >
      {{ submitting ? '保存中...' : '保存' }}
    </button>
  </form>
</template>

<script setup lang="ts">
import { useFormValidation } from '@/composables/useFormValidation'

const { errors, isFormValid, validate } = useFormValidation(formData, {
  path: [required(), validPath()],
  mapped_model_id: [required(), maxLength(100)],
  'config_groups[0].api_url': [required(), url()],
  'config_groups[0].api_key': [required(), minLength(10)],
})
</script>
```

---

### 🔴 问题 3.2: 用户体验流程断裂

**严重程度**: 🔴 高  
**位置**: 全局

#### 典型场景分析

##### 场景 1: 首次使用流程

```
❌ 当前流程:
启动应用 → Dashboard（空白） → 手动点击"配置管理" → 点击"新建配置" → 
填写复杂表单 → 保存 → 回到 Dashboard → 选择配置 → 启动

✅ 理想流程:
启动应用 → 欢迎向导（引导配置）→ 一键测试连接 → 自动启动服务
```

##### 场景 2: 服务启动失败

```
❌ 当前行为:
1. 显示 Toast 提示 "启动失败: permission denied"
2. 用户需自行排查原因
3. 端口可能已自动切换到 8443（静默）

✅ 理想行为:
1. 显示详细错误面板
2. 提供一键修复按钮（"使用备用端口 8443"）
3. 自动记录日志并提供查看入口
4. 给出明确的解决步骤指引
```

##### 场景 3: 配置编辑

```
❌ 当前行为:
1. 点击编辑按钮
2. 打开模态框
3. API Key 字段为空！（安全考虑但用户体验差）
4. 用户必须重新输入 API Key
5. 不确定是否覆盖原有 Key

✅ 理想行为:
1. 显示脱敏后的 Key：sk-****abcd
2. 提供"显示/隐藏"切换
3. 明确标注："留空则保持原值不变"
4. 提供"重新生成"选项
```

#### 改进建议

```typescript
// 引入向导模式
const useOnboarding = () => {
  const steps = [
    { id: 'welcome', component: WelcomeStep },
    { id: 'provider', component: ProviderSelectStep },
    { id: 'config', component: ApiKeyConfigStep },
    { id: 'test', component: ConnectionTestStep },
    { id: 'complete', component: CompleteStep },
  ]
  
  const currentStep = ref(0)
  const canNext = computed(() => validateStep(steps[currentStep]))
  
  return { steps, currentStep, next, prev, canNext }
}
```

---

### 🟡 问题 3.3: 响应式设计不足

**严重程度**: 🟡 中  
**位置**: [Dashboard.vue](gui/frontend/src/views/Dashboard.vue#L61)

#### 现状分析

```vue
<div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
  <div class="card col-span-2">  <!-- 主内容区固定占 2/3 -->
    <!-- ... -->
  </div>
  <div class="card">             <!-- 侧边栏固定占 1/3 -->
    <!-- 快捷操作 -->
  </div>
</div>
```

#### 问题说明

1. **断点单一**: 仅支持 `lg:` 断点，平板和手机适配差
2. **布局僵化**: 固定栅格比例无法适应不同屏幕尺寸
3. **字体/间距未适配**: 小屏幕上文字过小或过大
4. **触摸目标不足**: 按钮尺寸不适合移动设备

#### 改进建议

```vue
<template>
  <!-- 响应式布局 -->
  <div class="grid grid-cols-1 xl:grid-cols-3 gap-6">
    <main :class="[
      'space-y-6',
      { 'xl:col-span-2': !sidebarCollapsed }
    ]">
      <!-- 主内容 -->
    </main>
    
    <aside :class="[
      'hidden xl:block',  // 大屏显示侧边栏
      { 'fixed inset-y-0 right-0 w-80 z-40': sidebarOpen }  // 移动端抽屉
    ]">
      <!-- 侧边栏 -->
    </aside>
  </div>
</template>

<style>
/* 触摸友好 */
@media (pointer: coarse) {
  .btn-primary {
    min-height: 48px;  /* Apple HIG 标准 */
    padding: 12px 24px;
  }
  
  input, select {
    min-height: 44px;
  }
}
</style>
```

---

### 🟡 问题 3.4: 可访问性（A11y）缺失

**严重程度**: 🟡 中  
**位置**: 全局

#### 现状分析

```vue
<!-- ❌ 缺少语义化标签 -->
<div @click="handleStart">
  ▶ 启动
</div>

<!-- ❌ 缺少 ARIA 属性 -->
<input v-model="port" type="number" />

<!-- ❌ 缺少键盘导航支持 -->
<button @click.stop="handleEdit(config)">编辑</button>

<!-- ❌ 颜色对比度不足 -->
<span class="text-text-muted text-xs">{{ log.raw }}</span>
```

#### 改进建议

```vue
<!-- ✅ 语义化 HTML -->
<button 
  type="button"
  @click="handleStart"
  :disabled="startDisabled"
  aria-label="启动代理服务"
  aria-busy="loading"
>
  <span v-if="!loading">▶ 启动</span>
  <span v-else aria-hidden="true">⏳ 启动中...</span>
</button>

<!-- ✅ 表单关联 -->
<label for="port-input">监听端口</label>
<input
  id="port-input"
  v-model.number="port"
  type="number"
  min="1024"
  max="65535"
  aria-describedby="port-help"
  aria-invalid="!!portError"
/>
<span id="port-help" class="sr-only">端口号范围 1024-65535</span>

<!-- ✅ 键盘快捷键 -->
<div
  role="listbox"
  tabindex="0"
  @keydown.enter="selectItem"
  @keydown.escape="closeDropdown"
>
```

---

### 🟢 问题 3.5: 缺少加载状态和骨架屏

**严重程度**: 🟢 低  
**位置**: [ConfigManager.vue](gui/frontend/src/views/ConfigManager.vue#L274-L276)

#### 现状分析

```vue
<div v-if="configStore.loading" class="text-center py-12">
  加载中...  <!-- 仅文字提示 -->
</div>

<div v-else-if="configStore.configs.length === 0" class="text-center py-12">
  <!-- 空状态 -->
</div>
```

#### 改进建议

```vue
<template>
  <!-- 骨架屏 -->
  <div v-if="configStore.loading" class="space-y-3">
    <SkeletonCard v-for="i in 3" :key="i" />
  </div>
  
  <!-- 空状态引导 -->
  <EmptyState
    v-else-if="configs.length === 0"
    icon="📝"
    title="暂无配置"
    description="开始使用前需要创建一个代理配置"
    :actions="[
      { label: '新建配置', onClick: handleCreate, primary: true },
      { label: '导入配置', onClick: showImportModal }
    ]"
  >
    <template #footer>
      <a href="/docs/getting-started" target="_blank">查看快速入门指南 →</a>
    </template>
  </EmptyState>
</template>
```

---

## 4. 安全性问题

### 🔴 问题 4.1: API Key 明文存储

**严重程度**: 🔴 高  
**位置**: [gui/app.go:609-616](gui/app.go#L609-L616), [internal/config/config.go](internal/config/config.go)

#### 现状分析

```go
// TOML 文件明文存储
sb.WriteString("api_key = \"" + group.APIKey + "\"\n")

// 结果：
// api_key = "sk-proj-abc123456789xyz..."
```

#### 安全风险

1. **配置文件泄露风险**: 任何能读取文件系统的进程都能获取密钥
2. **版本控制系统**: 如果误提交到 Git，密钥永久泄露
3. **备份泄露**: 云备份可能包含敏感信息
4. **多用户环境**: 其他用户可能读取配置文件

#### 改进建议

```go
// 方案 A: 操作系统集成（推荐 Linux/macOS）
import "golang.org/x/crypto/ssh/terminal"

func EncryptForUser(plaintext string, userID string) (string, error) {
    // 使用用户的登录密钥加密
    // 或使用 keyring (gnome-keyring, macOS Keychain)
}

// 方案 B: 应用级加密
func EncryptAES(plaintext string, masterPassword string) (string, error) {
    // PBKDF2 + AES-GCM
}

// TOML 存储示例
// api_key = "enc:v1:aes256:gcmiv:encrypteddata"
```

---

### 🔴 问题 4.2: 认证机制过于简单

**严重程度**: 🔴 高  
**位置**: [internal/proxy/auth.go](internal/proxy/auth.go#L13-L22)

#### 现状分析

```go
func (a *ProxyAuth) Verify(authHeader string) bool {
    if a.AuthKey == "" {
        return true  // ⚠️ 空密钥 = 无认证！
    }
    provided := strings.TrimPrefix(authHeader, "Bearer ")
    return provided == a.AuthKey  // ⚠️ 明文比较，易受时序攻击
}
```

#### 安全风险

1. **默认无认证**: 空密钥时直接放行所有请求
2. **时序攻击风险**: 使用 `==` 比较字符串，可通过响应时间推断密钥
3. **无速率限制**: 可暴力破解密钥
4. **无 Token 过期**: 一旦获取，永久有效

#### 改进建议

```go
package auth

import (
    "crypto/subtle"
    "crypto/rand"
    "encoding/hex"
    "time"
    "golang.org/x/crypto/bcrypt"
)

type ProxyAuth struct {
    HashedKey     string // bcrypt hash
    EnableAuth    bool
    RateLimiter   *RateLimiter
    TokenManager  *TokenManager
}

func (a *ProxyAuth) Verify(authHeader string) bool {
    if !a.EnableAuth {
        return false // 默认要求认证
    }
    
    // 1. 时序安全的比较
    if subtle.ConstantTimeCompare([]byte(provided), []byte(a.HashedKey)) == 1 {
        return true
    }
    
    // 2. 速率限制
    if !a.RateLimiter.Allow(clientIP) {
        return false
    }
    
    return false
}

// JWT Token 支持
func (a *ProxyAuth) GenerateToken() (string, error) {
    claims := jwt.MapClaims{
        "exp": time.Now().Add(24 * time.Hour).Unix(),
        "iat": time.Now().Unix(),
    }
    return token.SignedString(claims)
}
```

---

### 🟡 问题 4.3: Hosts 文件修改权限问题

**严重程度**: 🟡 中  
**位置**: [internal/hosts/hosts.go](internal/hosts/hosts.go#L67-107)

#### 现状分析

```go
func (hm *HostsManager) AddEntry(logf func(string, ...interface{})) error {
    hostsFile := getHostsPath() // /etc/hosts (需要 root 权限)
    content, err := os.ReadFile(hostsFile)
    // ...
    err = os.WriteFile(hostsFile, []byte(buf.String()), 0644) // 0644 权限
}
```

#### 安全风险

1. **需要 root 权限**: 修改 `/etc/hosts` 需要特权
2. **竞态条件**: 并发读写可能导致数据损坏
3. **无回滚机制**: 写入失败后原始文件可能已损坏
4. **权限设置不当**: 0644 可能导致其他用户修改

#### 改进建议

```go
func (hm *HostsManager) AddEntry(logf func(string, ...interface{})) error {
    // 1. 原子写入
    tmpFile := hostsFile + ".tmp." + generateRandomSuffix()
    if err := os.WriteFile(tmpFile, content, 0644); err != nil {
        return err
    }
    
    // 2. 同步到磁盘
    if err := sync.File(tmpFd); err != nil {
        os.Remove(tmpFile)
        return err
    }
    
    // 3. 原子重命名
    if err := os.Rename(tmpFile, hostsFile); err != nil {
        os.Remove(tmpFile)
        return err
    }
    
    return nil
}
```

---

## 5. 性能与可扩展性问题

### 🟡 问题 5.1: 轮询机制效率低下

**严重程度**: 🟡 中  
**位置**: [stores/proxy.ts:198-216](gui/frontend/src/stores/proxy.ts#L198-216)

#### 现状分析

```typescript
startPolling(intervalMs: number = 3000) {
  this._pollingTimer = setInterval(() => {
    this._syncMetaFromBackend()  // 每次 2 次 RPC 调用
    this.fetchLogs()              // 获取全部日志
  }, intervalMs)
}
```

#### 性能影响

1. **频繁 RPC 调用**: 每 3 秒 2 次跨进程通信（Wails Bindings）
2. **全量数据传输**: 每次获取完整状态和日志，增量更新更高效
3. **资源浪费**: 即使无变化也持续轮询
4. **移动端耗电**: 持续网络活动消耗电量

#### 改进方案

```typescript
// 方案 A: WebSocket 实时推送（最佳）
startWebSocket() {
  this.ws = new WebSocket('ws://localhost:events')
  this.ws.onmessage = (event) => {
    const update = JSON.parse(event.data)
    switch(update.type) {
      case 'status_changed':
        this.updateStatus(update.payload)
        break
      case 'log_added':
        this.appendLog(update.payload)
        break
    }
  }
}

// 方案 B: 长轮询优化
async pollWithBackoff() {
  let delay = 1000
  
  while (this._pollingActive) {
    const result = await this.fetchUpdates(this.lastUpdateId)
    
    if (result.hasChanges) {
      delay = 1000  // 有更新，加快频率
      this.applyUpdates(result.changes)
    } else {
      delay = Math.min(delay * 1.5, 30000)  // 无更新，退避
    }
    
    await sleep(delay)
  }
}
```

---

### 🟡 问题 5.2: 内存泄漏风险

**严重程度**: 🟡 中  
**位置**: [stores/proxy.ts:68-76](gui/frontend/src/stores/proxy.ts#L68-76)

#### 现状分析

```typescript
addLog(raw: string) {
  const entry = this.parseLog(raw)
  if (entry) {
    this.logs.push(entry)  // 无限增长
    if (this.logs.length > this.maxLogs) {
      this.logs = this.logs.slice(-this.maxLogs)  // 创建新数组
    }
  }
}
```

#### 问题说明

1. **数组频繁重建**: `slice()` 创建新数组，触发 GC
2. **对象未释放**: LogEntry 对象可能被外部引用
3. **无 LRU 策略**: 先进先出可能淘汰重要日志

#### 改进建议

```typescript
// 使用环形缓冲区
class CircularBuffer<T> {
  private buffer: (T | undefined)[]
  private head = 0
  private size = 0
  
  push(item: T) {
    const idx = (this.head + this.size) % this.capacity
    this.buffer[idx] = item
    
    if (this.size < this.capacity) {
      this.size++
    } else {
      this.head = (this.head + 1) % this.capacity
    }
  }
  
  toArray(): T[] {
    const result: T[] = []
    for (let i = 0; i < this.size; i++) {
      const idx = (this.head + i) % this.capacity
      if (this.buffer[idx]) result.push(this.buffer[idx]!)
    }
    return result
  }
}
```

---

## 6. 代码质量问题

### 🟡 问题 6.1: TypeScript 类型定义不完整

**严重程度**: 🟡 中  
**位置**: [types/index.ts](gui/frontend/src/types/index.ts), [runtime.ts](gui/frontend/src/utils/runtime.ts)

#### 现状分析

```typescript
// types/index.ts - 类型定义
export interface ConfigData {
  path: string
  mapped_model_id: string
  auth_key: string
  config_groups: ConfigGroupData[]
}

// runtime.ts - 函数签名
export const GetSupportedProviders = () => getWailsFunc('GetSupportedProviders')()
// 返回类型是 any[]!
```

#### 问题说明

1. **Provider 类型缺失**: 使用 `any[]` 导致 IDE 无法提供补全
2. **后端返回值未约束**: 所有 Wails Binding 返回 `any`
3. **运行时错误风险**: 类型错误只能在运行时发现

#### 改进建议

```typescript
// 完整的类型定义
interface ProviderInfo {
  id: ProviderType
  name: string
  default_url: string
  default_route: string
  models: Model[]
  api_key_hint: string
  features: ProviderFeature[]
}

type ProviderType =
  | 'openai_chat_completion'
  | 'openai_response'
  | 'anthropic'
  | 'gemini'
  | 'openrouter'

interface Model {
  id: string
  name: string
  context_window?: number
  pricing?: PricingInfo
}

// 强类型函数签名
declare module '@/utils/runtime' {
  export function GetSupportedProviders(): Promise<ProviderInfo[]>
  export function CreateConfig(data: ConfigData): Promise<string | null>
  export function TestConnection(path: string): Promise<TestResult>
}
```

---

### 🟢 问题 6.2: 缺少单元测试

**严重程度**: 🟢 低  
**位置**: 全局

#### 测试覆盖率估算

| 模块 | 估算覆盖率 | 关键缺失 |
|------|-----------|---------|
| internal/config | ~60% | 边界情况测试 |
| internal/cert | ~50% | 证书链验证 |
| internal/proxy | ~30% | HTTP handler 测试 |
| internal/platform | ~20% | 平台特定逻辑 |
| gui/app.go | 0% | 全部未测试 |
| frontend | 0% | 组件测试 |

#### 改进建议

```go
// 示例：配置加载测试
func TestConfigLoad_InvalidProvider(t *testing.T) {
  tests := []struct{
    name    string
    toml    string
    wantErr bool
  }{
    {
      name: "empty provider",
      toml: `
        provider = ""
        api_url = "https://example.com"
      `,
      wantErr: true,
    },
    {
      name: "unsupported provider",
      toml: `
        provider = "unknown_provider"
      `,
      wantErr: true,
    },
  }
  
  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      cfg, err := config.LoadFromTOML([]byte(tt.toml))
      if (err != nil) != tt.wantErr {
        t.Errorf("LoadFromTOML() error = %v, wantErr %v", err, tt.wantErr)
      }
    })
  }
}
```

---

## 7. 改进建议优先级矩阵

### 🎯 P0 - 立即修复（阻塞性问题）

| # | 问题 | 工作量 | 影响 |
|---|------|--------|------|
| 1 | **表单验证缺失** | 2天 | 数据完整性、用户体验 |
| 2 | **API Key 安全存储** | 3天 | 安全合规 |
| 3 | **错误处理统一** | 2天 | 可维护性、调试效率 |

### 🎯 P1 - 短期改进（1-2 周）

| # | 问题 | 工作量 | 影响 |
|---|------|--------|------|
| 4 | **首次使用向导** | 3天 | 新手体验 |
| 5 | **认证机制增强** | 2天 | 安全性 |
| 6 | **轮询机制优化** | 2天 | 性能 |
| 7 | **TypeScript 类型完善** | 1天 | 开发效率 |

### 🎯 P2 - 中期优化（2-4 周）

| # | 问题 | 工作量 | 影响 |
|---|------|--------|------|
| 8 | **响应式设计重构** | 3天 | 移动端支持 |
| 9 | **状态管理重构** | 3天 | 可维护性 |
| 10 | **单元测试补充** | 5天 | 代码质量 |
| 11 | **架构分层清晰化** | 5天 | 可扩展性 |

### 🎯 P3 - 长期规划（1-2 月）

| # | 问题 | 工作量 | 影响 |
|---|------|--------|------|
| 12 | **可访问性全面优化** | 3天 | 合规性、包容性 |
| 13 | **国际化支持（i18n）** | 5天 | 全球市场 |
| 14 | **插件系统** | 10天 | 生态扩展 |
| 15 | **Web 管理界面** | 14天 | 远程管理 |

---

## 8. 实施路线图

### 📅 Phase 1: 稳定性提升（Week 1-2）

```
目标: 解决核心功能和关键安全问题

Week 1:
├─ Day 1-2: 统一错误处理机制
├─ Day 3-4: 实现表单验证系统
└─ Day 5: API Key 加密存储（初步）

Week 2:
├─ Day 1-2: 认证机制增强
├─ Day 3-4: 首次使用向导 MVP
└─ Day 5: 测试和 Bug 修复
```

### 📅 Phase 2: 体验优化（Week 3-4）

```
目标: 提升 UX 和性能

Week 3:
├─ Day 1-2: 轮询机制改为事件推送
├─ Day 3-4: TypeScript 类型完善
└─ Day 5: 骨架屏和加载状态

Week 4:
├─ Day 1-2: 响应式布局优化
├─ Day 3-4: 状态管理重构
└─ Day 5: 用户测试和反馈收集
```

### 📅 Phase 3: 质量保障（Week 5-8）

```
目标: 达到生产就绪状态

Week 5-6:
├─ 单元测试覆盖率达到 70%
├─ 集成测试（E2E）
└─ 性能基准测试

Week 7-8:
├─ 安全审计
├─ 文档完善
└─ 发布准备
```

---

## 📊 总结

### 核心发现

OpenHijack 作为一个本地 HTTPS 代理工具，**技术选型合理、核心功能完备**，但在以下方面需要重点改进：

1. **🏗️ 架构层面**: 需要清晰的分层和依赖注入
2. **🎨 UI 层面**: 表单验证和用户引导是当务之急
3. **🔒 安全层面**: 密钥存储和认证机制亟需加强
4. **⚡ 性能层面**: 轮询机制应改为实时推送

### 下一步行动

建议按照 **P0 → P1 → P2 → P3** 的优先级顺序逐步实施改进，预计 **4-8 周**内可以显著提升项目的成熟度和用户体验。

---

**报告生成时间**: 2026-05-07  
**审查工具**: AI Code Review Assistant  
**下次审查建议**: Phase 1 完成后进行回归审查
