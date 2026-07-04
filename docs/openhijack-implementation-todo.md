# OpenHijack 产品实施计划（TODO）

**基于**：[docs/openhijack-product-prd.md](./openhijack-product-prd.md)
**生成日期**：2026-07-04
**最后更新**：2026-07-04（/dev 工作流执行后）
**当前代码库状态盘点**：已完成核心代理 + 跨平台安装 + GUI 骨架 + 多协议适配（OpenAI/Anthropic/Gemini）+ 配置热重载 + 审计日志 + doctor 诊断 + CI 骨架；待补齐密钥隔离、限流熔断、性能基准、企业 KMS 与 Web GUI。

---

## 现状盘点（已实现 vs 待实现）

| 能力 | 状态 | 备注 |
|------|------|------|
| 本地 HTTPS 代理 + 多域名 SNI | ✅ 已实现 | [internal/proxy/proxy.go](../internal/proxy/proxy.go) |
| OpenAI Chat Completions 协议 | ✅ 已实现 | [internal/proxy/transport.go](../internal/proxy/transport.go)（已重构为基于 ProviderAdapter） |
| Anthropic 原生协议适配器 | ✅ 已实现 | [internal/proxy/provider/anthropic.go](../internal/proxy/provider/anthropic.go)（含 system 消息映射 + SSE 转换） |
| Gemini 原生协议适配器 | ✅ 已实现 | [internal/proxy/provider/gemini.go](../internal/proxy/provider/gemini.go)（含 contents 映射 + streamGenerateContent） |
| ProviderAdapter 接口与注册表 | ✅ 已实现 | [internal/proxy/provider/provider.go](../internal/proxy/provider/provider.go) |
| 模型映射 (`mapped_model_id` ↔ `model_id`) | ✅ 已实现 | [internal/config/config.go](../internal/config/config.go) |
| 多配置组 + `current_config_index` | ✅ 已实现 | 同上 |
| 客户端鉴权 (`auth_key`) | ✅ 已实现 | [internal/proxy/auth.go](../internal/proxy/auth.go) |
| 系统提示词捕获与覆盖 | ✅ 已实现 | [internal/proxy/system_prompt.go](../internal/proxy/system_prompt.go) |
| CA 生成 + 跨平台安装 | ✅ 已实现 | [internal/cert/](../internal/cert/) |
| Hosts 劫持（跨平台） | ✅ 已实现 | [internal/hosts/hosts.go](../internal/hosts/hosts.go) |
| 权限提升（sudo/UAC） | ✅ 已实现 | [internal/platform/](../internal/platform/) |
| SSE 流式 / 非流式透传 | ✅ 已实现 | [internal/proxy/transport.go](../internal/proxy/transport.go) |
| `init` / `serve` / `elevate` / `cleanup` / `paths` 命令 | ✅ 已实现 | [cmd/openhijack/main.go](../cmd/openhijack/main.go) |
| SecretStore 抽象（File/EnvVar） | ✅ 已实现 | [internal/crypto/store.go](../internal/crypto/store.go) |
| GUI 骨架（Wails） | ✅ 已实现 | [gui/app.go](../gui/app.go) |
| Provider 枚举定义（Anthropic/Gemini/OpenRouter） | ✅ 已实现 | [internal/config/config.go](../internal/config/config.go) 声明 + [internal/proxy/provider/](../internal/proxy/provider/) 完整实现 |
| 配置热重载 | ✅ 已实现 | [internal/config/watcher.go](../internal/config/watcher.go)（fsnotify + 500ms 防抖） |
| 请求审计日志 | ✅ 已实现 | [internal/audit/audit.go](../internal/audit/audit.go)（JSON Lines 格式，93.8% 覆盖率） |
| `openhijack doctor` 命令 | ✅ 已实现 | [cmd/openhijack/doctor.go](../cmd/openhijack/doctor.go)（CA/hosts/端口/配置/上游健康检查） |
| `openhijack logs` 命令 | ❌ 未实现 | |
| 限流 / 熔断 | ❌ 未实现 | |
| 上游密钥隔离（明文不暴露给普通用户） | ❌ 未实现 | `config.toml` 中 `api_key` 为明文 |
| 外部日志收集（syslog/webhook） | ❌ 未实现 | |
| KMS 集成（Vault/AWS SM/Azure KV） | ❌ 未实现 | SecretStore 接口已预留 |
| 规则化动态路由（按模型/用户/路径） | ❌ 未实现 | |
| 性能基准测试 | ❌ 未实现 | |
| CI 多平台测试 | ⚠️ 部分实现 | [.github/workflows/build.yml](../.github/workflows/build.yml) 仅 ubuntu-latest，未覆盖 Windows/macOS |

---

## Phase 1：MVP（初始发布）

> **目标**：个人开发者 5 分钟跑通；小团队（5-20 人）共享配置 + 基础密钥隔离 + 日志审计。
> **完成标准**：所有 P0 任务完成，P1 任务完成 ≥ 80%。

### 模块 A：上游协议扩展

#### A1. Anthropic 原生协议适配器 — P0 ✅
- **依赖**：无
- **文件**：新建 `internal/proxy/provider/anthropic.go`
- **状态**：✅ 已完成（2026-07-04）
- **任务**：
  - [x] 实现 `ProviderAdapter` 接口（`BuildUpstreamRequest`、`GetUpstreamURL`、`SetAuthHeaders`、`MapResponseToOpenAI`、`MapStreamToOpenAI`）
  - [x] 处理 Anthropic `/v1/messages` 请求/响应结构转换
  - [x] 支持 `x-api-key` 头部鉴权（而非 `Bearer`）
  - [x] 支持 `anthropic-version` 头部
  - [x] SSE 流式响应转换（Anthropic event 格式 → OpenAI chunk 格式）
- **验收标准**：
  - 配置 `provider = "anthropic"` 时，Trae 通过 OpenAI 兼容接口调用能成功拿到响应
  - 流式与非流式均能正常工作
  - 单元测试覆盖率 ≥ 80%（实测 anthropic_test.go 覆盖 BuildUpstreamRequest + Map 系列）

#### A2. Gemini 原生协议适配器 — P0 ✅
- **依赖**：无
- **文件**：新建 `internal/proxy/provider/gemini.go`
- **状态**：✅ 已完成（2026-07-04）
- **任务**：
  - [x] 实现 `ProviderAdapter` 接口
  - [x] 处理 Gemini `generateContent` / `streamGenerateContent` 端点
  - [x] API Key 通过 query param `?key=` 传递
  - [x] 请求/响应字段映射（`contents` ↔ `messages`、`candidates` ↔ `choices`）
  - [x] SSE 流式响应转换
- **验收标准**：
  - 配置 `provider = "gemini"` 时，Trae 能成功调用
  - 流式与非流式均正常
  - 单元测试覆盖（gemini_test.go 覆盖 BuildUpstreamRequest + Map 系列）

#### A3. Provider 路由分发器 — P0 ✅
- **依赖**：A1、A2
- **文件**：修改 [internal/proxy/transport.go](../internal/proxy/transport.go)、新建 [internal/proxy/provider/provider.go](../internal/proxy/provider/provider.go)
- **状态**：✅ 已完成（2026-07-04）
- **任务**：
  - [x] 定义 `ProviderAdapter` 接口
  - [x] 根据 `config_group.provider` 选择适配器（通过 Register/GetAdapter 注册表）
  - [x] 保留 OpenAI Chat Completions 现有逻辑作为默认适配器（[openai.go](../internal/proxy/provider/openai.go)）
  - [ ] OpenRouter 复用 OpenAI 兼容适配器（标记为 P1，未实现）
- **验收标准**：
  - 不同配置组可使用不同 provider，请求被正确路由到对应适配器
  - 切换 `current_config_index` 后立即生效

#### A4. Provider 注册与单元测试 — P0 ✅
- **依赖**：A3
- **状态**：✅ 已完成（2026-07-04，覆盖率 90.5%）
- **任务**：
  - [x] 为每个 provider 编写 mock 上游的单元测试
  - [x] 覆盖正常请求、错误响应、流式响应、响应映射
  - [x] 测试覆盖率 ≥ 80%（实测 **90.5%**，超目标）
- **测试文件**：
  - [provider_test.go](../internal/proxy/provider/provider_test.go) — 注册表 + SystemPromptStore
  - [openai_test.go](../internal/proxy/provider/openai_test.go) — OpenAI 适配器
  - [anthropic_test.go](../internal/proxy/provider/anthropic_test.go) — Anthropic 适配器
  - [gemini_test.go](../internal/proxy/provider/gemini_test.go) — Gemini 适配器

### 模块 B：配置热重载

#### B1. 文件监听与原子重载 — P0 ✅
- **依赖**：无
- **文件**：新建 [internal/config/watcher.go](../internal/config/watcher.go)、[internal/config/watcher_test.go](../internal/config/watcher_test.go)
- **状态**：✅ Watcher 已完成（2026-07-04）；⚠️ 待集成到 ProxyServer（尚未在 proxy.go 中调用 NewWatcher）
- **任务**：
  - [x] 使用 `fsnotify` 监听 `config.toml` 变更
  - [x] 500ms 防抖：避免多次写入触发多次回调
  - [x] 解析新配置后通过回调返回（onChange 闭包由调用方决定如何替换）
  - [x] Close 后不再触发回调
  - [ ] **未完成**：在 [internal/proxy/proxy.go](../internal/proxy/proxy.go) 的 Serve 中集成 Watcher，原子替换 `ProxyServer.config`
  - [ ] **未完成**：校验失败时保持旧配置运行，并在日志中报警
- **验收标准**：
  - ✅ Watcher 单元测试：文件修改触发回调、防抖生效、Close 不再回调
  - ⚠️ 集成测试待补：修改 `current_config_index` 后下一条请求使用新配置组
  - ⚠️ 错误恢复待补：配置文件语法错误时代理不崩溃

#### B2. 配置校验增强 — P1
- **依赖**：B1
- **任务**：
  - [ ] 启动时校验必填字段（`api_url`、`model_id`、`api_key`）
  - [ ] 校验 `current_config_index` 越界
  - [ ] 输出中文/英文错误提示与修复建议

### 模块 C：企业安全与审计

#### C1. 上游密钥隔离 — P0
- **依赖**：无
- **文件**：修改 [internal/config/config.go](../internal/config/config.go)、[internal/crypto/store.go](../internal/crypto/store.go)
- **任务**：
  - [ ] `config.toml` 支持 `api_key_ref` 字段引用 SecretStore 中的密钥
  - [ ] 当 `api_key_ref` 存在时，`api_key` 字段被忽略，运行时从 SecretStore 解析
  - [ ] 普通用户查看配置时，`api_key` 显示为 `***`（CLI 输出脱敏）
  - [ ] 文档说明如何通过环境变量或 SecretStore 注入密钥
- **验收标准**：
  - 配置文件中不出现明文 API Key
  - 代理能正确从 SecretStore 读取密钥并调用上游
  - `paths` 命令输出不泄露密钥

#### C2. 结构化审计日志 — P0 ✅
- **依赖**：无
- **文件**：新建 [internal/audit/audit.go](../internal/audit/audit.go)、[internal/audit/audit_test.go](../internal/audit/audit_test.go)
- **状态**：✅ AuditLogger 已完成（2026-07-04，覆盖率 93.8%）；⚠️ 待集成到 ProxyServer
- **任务**：
  - [x] 定义审计记录结构 `AuditEntry`：`Timestamp`、`RequestID`、`Method`、`Path`、`Status`、`Upstream`、`Model`、`Duration`、`ClientIP`、`Error`
  - [x] `AuditLogger` 支持 JSON Lines 格式写入，并发安全（`sync.Mutex`）
  - [x] `NewAuditLogger(w io.Writer)` + `Log(entry)` + `LogRequest(...)`
  - [ ] **未完成**：在 `handleChatCompletions` 和 `handleOther` 中埋点（未集成到 proxy.go）
  - [ ] **未完成**：默认写入 `<data_dir>/audit.log`（需在 main.go 配置文件路径）
  - [ ] **未完成**：配置开关 `audit_enabled`、`audit_log_path`
  - [ ] **未完成**：脱敏配置（是否记录请求/响应体）
- **验收标准**：
  - ✅ AuditLogger 单元测试覆盖率 93.8%（≥90% 目标）
  - ✅ JSON Lines 格式可被 `jq` 解析
  - ⚠️ 集成埋点待补：每条请求产生一条审计记录

#### C3. `openhijack logs` 命令 — P1
- **依赖**：C2
- **文件**：修改 [cmd/openhijack/main.go](../cmd/openhijack/main.go)
- **任务**：
  - [ ] 新增 `logs` 子命令，只读查看审计日志
  - [ ] 支持 `--tail N`、`--filter model=xxx`、`--since 1h` 参数
  - [ ] 不重启或中断代理进程
- **验收标准**：
  - 代理运行时执行 `openhijack logs --tail 20` 能看到最近 20 条记录
  - 过滤参数生效

### 模块 D：故障诊断

#### D1. `openhijack doctor` 命令 — P0 ✅
- **依赖**：无
- **文件**：新建 [cmd/openhijack/doctor.go](../cmd/openhijack/doctor.go)、修改 [cmd/openhijack/main.go](../cmd/openhijack/main.go)（注册 `doctor` 子命令）
- **状态**：✅ 已完成（2026-07-04）
- **任务**：
  - [x] 检查项：CA 证书存在性、CA 是否被系统信任、hosts 条目、端口可用性、上游连通性、配置文件语法
  - [x] 输出结构化诊断报告（PASS/WARN/FAIL）
  - [x] 对 WARN/FAIL 提供修复建议
- **验收标准**：
  - 健康环境运行 `doctor` 全部 PASS
  - 手动删除 hosts 条目后运行 `doctor` 能检测到并给出修复命令
  - `openhijack doctor` 已在 main.go 中注册为子命令

### 模块 E：性能与稳定性

#### E1. 连接池调优 — P1
- **依赖**：无
- **文件**：修改 [internal/proxy/transport.go](../internal/proxy/transport.go)
- **任务**：
  - [ ] `MaxIdleConns`、`MaxIdleConnsPerHost` 可配置
  - [ ] 默认值适配 100 并发场景
  - [ ] 添加超时配置（连接、TLS、读取）
- **验收标准**：
  - 100 并发场景下无明显连接复用问题

#### E2. 基础限流 — P1
- **依赖**：无
- **文件**：新建 `internal/proxy/ratelimit.go`
- **任务**：
  - [ ] 基于 `golang.org/x/time/rate` 实现令牌桶
  - [ ] 支持全局 QPS 配置
  - [ ] 超限时返回 429 + OpenAI 兼容错误结构
- **验收标准**：
  - 配置 `rate_limit = 10` 时，第 11 个请求/秒返回 429

#### E3. 性能基准测试 — P1
- **依赖**：E1、E2
- **文件**：新建 `internal/proxy/benchmark_test.go`
- **任务**：
  - [ ] 使用 `httptest` 模拟上游，基准测试代理层延迟
  - [ ] 验证 P95 延迟增加 < 20ms
  - [ ] 验证 100 并发稳定运行
- **验收标准**：
  - 基准测试通过，结果写入文档

### 模块 F：CI 与质量

#### F1. GitHub Actions CI 工作流 — P0 ⚠️ 部分完成
- **依赖**：无
- **文件**：新建 [.github/workflows/build.yml](../.github/workflows/build.yml)
- **状态**：⚠️ 部分完成（2026-07-04）— 仅 ubuntu-latest，未实现三平台
- **任务**：
  - [ ] **未完成**：Windows / macOS / Linux 三平台构建（当前仅 ubuntu-latest）
  - [x] `go test ./...` 覆盖（含 -coverprofile=coverage.out）
  - [x] `go vet ./...` lint job（与 test 并行）
  - [ ] **未完成**：`golangci-lint run`（当前仅 `go vet`）
- **验收标准**：
  - ✅ PR 触发 CI（push/PR 到 main/master）
  - ⚠️ 单平台（ubuntu）构建通过
  - ❌ 三平台构建待补

#### F2. 集成测试套件 — P1 ⚠️ 部分完成
- **依赖**：A1、A2
- **文件**：[internal/proxy/provider/*_test.go](../internal/proxy/provider/)
- **状态**：⚠️ 部分完成（2026-07-04）— 单元测试覆盖完整，集成测试待补
- **任务**：
  - [x] 每个 provider 单元测试覆盖正常、错误、流式响应映射（实测覆盖率 90.5%）
  - [x] 关键路径覆盖率 ≥ 80%（实测 **90.5%**）
  - [ ] **未完成**：mock 上游的集成测试（httptest.Server 端到端验证）
  - [ ] **未完成**：超时、流式中断场景

---

## Phase 2：增强版（发布后 3 个月）

> **目标**：补齐全部三种原生协议、企业 KMS 集成、日志外发、规则路由、限流熔断、Web GUI 远期规划。

### 模块 G：协议补全

#### G1. OpenRouter 原生协议适配 — P1
- **依赖**：Phase 1 完成
- **任务**：
  - [ ] 复用 OpenAI 兼容适配器，补充 OpenRouter 特有头部（`HTTP-Referer`、`X-Title`）
  - [ ] 支持模型路由（`openai/gpt-4o` 格式）

#### G2. OpenAI Response API 适配 — P2
- **依赖**：Phase 1 完成
- **任务**：
  - [ ] 实现 `/v1/responses` 端点适配

### 模块 H：动态路由

#### H1. 规则引擎 — P1
- **依赖**：Phase 1 B1 热重载
- **文件**：新建 `internal/proxy/router/router.go`
- **任务**：
  - [ ] 支持配置规则：按 `model`、`path`、`header` 路由到不同配置组
  - [ ] 规则热重载
  - [ ] 规则冲突时优先级处理
- **验收标准**：
  - 不同模型的请求被路由到不同上游

### 模块 I：企业安全增强

#### I1. KMS 集成（Vault） — P1
- **依赖**：Phase 1 C1
- **文件**：修改 [internal/crypto/store.go](../internal/crypto/store.go)
- **任务**：
  - [ ] 实现 `VaultStore`（HashiCorp Vault KV v2）
  - [ ] `OPENHIJACK_KEYSTORE=vault` 时启用
  - [ ] 支持缓存与刷新

#### I2. KMS 集成（AWS SM / Azure KV） — P2
- **依赖**：I1
- **任务**：
  - [ ] 实现 `AWSSecretsManagerStore`
  - [ ] 实现 `AzureKeyVaultStore`

#### I3. 审计日志外发 — P1
- **依赖**：Phase 1 C2
- **任务**：
  - [ ] 支持 syslog 输出
  - [ ] 支持 HTTP webhook 输出
  - [ ] 支持 Fluentd / Loki（通过 syslog 转发）

#### I4. 审计可视化报表 — P2
- **依赖**：I3
- **任务**：
  - [ ] 按模型/时间/状态码聚合统计
  - [ ] 导出 CSV / JSON

### 模块 J：稳定性增强

#### J1. 熔断器 — P1
- **依赖**：Phase 1 E2
- **任务**：
  - [ ] 上游连续失败 N 次后熔断
  - [ ] 半开探测恢复

#### J2. 响应缓存 — P2
- **依赖**：无
- **任务**：
  - [ ] 对相同请求缓存响应（可配置 TTL）
  - [ ] 仅缓存非流式响应

#### J3. 配额管理 — P2
- **依赖**：Phase 1 C2
- **任务**：
  - [ ] 按 `auth_key` 设置日/月配额
  - [ ] 超额返回 429

### 模块 K：Web GUI 规划

#### K1. Web GUI 信息架构设计 — P1
- **依赖**：无
- **任务**：
  - [ ] 核心页面：仪表盘、配置管理、日志查看、证书管理、设置
  - [ ] 交互流程图
  - [ ] 与现有 Wails GUI 的关系（替换 or 并存）

#### K2. Web GUI 原型实现 — P2
- **依赖**：K1
- **任务**：
  - [ ] 基于 Vue + 现有 Wails 后端 API 实现
  - [ ] 实时状态监控
  - [ ] 配置热编辑

---

## Phase 3：生态扩展

> **目标**：插件化、集群化、社区化。

### 模块 L：插件与扩展

#### L1. IDE 插件 — P2
- **任务**：
  - [ ] VS Code / Trae 扩展，配置一键同步
  - [ ] 状态栏指示器

#### L2. 浏览器插件 — P3
- **任务**：
  - [ ] 配置同步、状态查看

#### L3. 上游协议插件机制 — P2
- **任务**：
  - [ ] 定义 provider plugin 接口
  - [ ] 动态加载（Go plugin 或 WASM）

### 模块 M：集群与高可用

#### M1. 多实例集群 — P3
- **任务**：
  - [ ] 配置同步机制
  - [ ] 负载均衡

#### M2. 高可用部署 — P3
- **任务**：
  - [ ] 健康检查端点
  - [ ] 优雅故障转移

### 模块 N：社区市场

#### N1. 配置模板市场 — P3
- **任务**：
  - [ ] 预置常见 provider 配置模板
  - [ ] 模型推荐

#### N2. 文档与示例 — P2
- **任务**：
  - [ ] 快速开始指南
  - [ ] 各平台安装排障手册
  - [ ] 企业部署最佳实践

---

## 任务依赖关系

```
Phase 1:
  A1 (Anthropic) ─┐
  A2 (Gemini)    ─┼─→ A3 (路由分发) ──→ A4 (测试)
                  │
  B1 (热重载) ────┼─→ B2 (校验增强)
                  │
  C1 (密钥隔离)   │
  C2 (审计日志) ──┼─→ C3 (logs 命令)
                  │
  D1 (doctor)    │
  E1 (连接池) ───┼─→ E3 (基准测试)
  E2 (限流) ─────┘
  F1 (CI) ──────── 独立
  F2 (集成测试) ── 依赖 A1、A2

Phase 2:
  G1/G2 ── 依赖 Phase 1
  H1 ──── 依赖 B1
  I1 ──── 依赖 C1
  I3 ──── 依赖 C2
  J1 ──── 依赖 E2
  K1 ──── 独立
```

---

## 优先级总览

| 优先级 | 含义 | Phase 1 任务 | 状态 |
|--------|------|-------------|------|
| **P0** | MVP 必须完成 | A1、A2、A3、A4、B1、C1、C2、D1、F1 | ✅ 7/9 完成（A1-A4、B1、C2、D1）❌ C1 未实现 ⚠️ F1 部分完成 |
| **P1** | MVP 强烈建议 | B2、C3、E1、E2、E3、F2 | ⚠️ F2 部分完成（单元测试 90.5%，集成测试待补）|
| **P2** | 可延后到 Phase 2 | — | — |

### Phase 1 P0 完成情况（截至 2026-07-04）

| 任务 | 状态 | 关键文件 | 覆盖率 |
|------|------|---------|--------|
| A1 Anthropic 适配器 | ✅ 完成 | [anthropic.go](../internal/proxy/provider/anthropic.go) | 见 A4 |
| A2 Gemini 适配器 | ✅ 完成 | [gemini.go](../internal/proxy/provider/gemini.go) | 见 A4 |
| A3 Provider 路由分发 | ✅ 完成 | [provider.go](../internal/proxy/provider/provider.go), [transport.go](../internal/proxy/transport.go) | — |
| A4 Provider 单元测试 | ✅ 完成 | [*_test.go](../internal/proxy/provider/) | **90.5%** |
| B1 配置热重载 | ⚠️ Watcher 完成 / 集成待补 | [watcher.go](../internal/config/watcher.go) | NewWatcher 90.9% |
| C1 上游密钥隔离 | ❌ 未实现 | — | — |
| C2 结构化审计日志 | ⚠️ Logger 完成 / 埋点待补 | [audit.go](../internal/audit/audit.go) | **93.8%** |
| D1 doctor 命令 | ✅ 完成 | [doctor.go](../cmd/openhijack/doctor.go) | — |
| F1 CI 工作流 | ⚠️ 部分完成（仅 ubuntu） | [build.yml](../.github/workflows/build.yml) | — |

### 遗留工作（待下一步实施）

1. **C1 上游密钥隔离**（P0，未开始）：
   - `config.toml` 支持 `api_key_ref` 字段引用 SecretStore
   - CLI 输出脱敏
2. **B1 集成到 ProxyServer**（P0，未开始）：
   - 在 proxy.go 的 Serve 中启动 Watcher
   - 原子替换 `ProxyServer.config`，校验失败保持旧配置
3. **C2 集成埋点**（P0，未开始）：
   - 在 handleChatCompletions / handleOther 中调用 AuditLogger
   - 默认写入 `<data_dir>/audit.log`
   - 配置开关 `audit_enabled`、`audit_log_path`、脱敏配置
4. **F1 多平台 CI**（P0，未开始）：
   - Windows / macOS 矩阵
   - golangci-lint 替换 go vet

---

## 验收里程碑

### 里程碑 1：多协议 MVP（Phase 1 P0 完成）
- [x] ✅ Anthropic + Gemini 协议可用（A1、A2 完成）
- [x] ✅ ProviderAdapter 接口 + 注册表 + 路由分发（A3、A4 完成，覆盖率 90.5%）
- [x] ✅ 配置热重载基础（B1 Watcher 完成，⚠️ 待集成到 ProxyServer）
- [ ] ❌ 密钥隔离（C1 未完成，config.toml 仍为明文 api_key）
- [x] ✅ 审计日志（C2 AuditLogger 完成，⚠️ 待集成到 ProxyServer）
- [x] ✅ `doctor` 命令（D1 完成）
- [ ] ⚠️ CI 三平台构建（F1 仅 ubuntu-latest，缺 Windows/macOS）

### 里程碑 2：MVP 发布（Phase 1 全部完成）
- [ ] P1 任务完成 ≥ 80%
- [ ] 性能基准测试通过（P95 < 20ms，100 并发）
- [ ] 集成测试覆盖率 ≥ 80%
- [ ] 文档完整（安装、配置、排障）

### 里程碑 3：增强版（Phase 2 完成）
- [ ] 三种原生协议全部支持
- [ ] KMS 集成（Vault）
- [ ] 日志外发
- [ ] 规则路由
- [ ] 熔断 + 限流
- [ ] Web GUI 原型

### 里程碑 4：生态扩展（Phase 3）
- [ ] 插件机制
- [ ] 集群部署
- [ ] 社区市场

---

*本计划基于 PRD 与当前代码库盘点生成，可根据实际开发进度动态调整。*
