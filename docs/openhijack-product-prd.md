# Product Requirements Document: OpenHijack

**Version**: 1.0
**Date**: 2026-07-04
**Author**: Sarah (Product Owner)
**Quality Score**: 93/100

---

## Executive Summary

OpenHijack 是一款本地 HTTPS 代理服务器，旨在解决固定服务商客户端（如 Trae IDE）只能连接 OpenAI、OpenRouter 等预设上游的问题。通过 hosts 劫持 + 本地自签名 CA + HTTPS 代理转发，OpenHijack 让用户无需修改客户端代码，即可把请求转发到自选的实际上游服务（私有模型、企业网关、其他云服务商等）。

本产品同时服务于两类核心用户：
1. **个人开发者**：希望在一个 IDE 中灵活切换不同模型或上游，而不受客户端预设服务商限制。
2. **团队/企业管理员**：需要为成员统一配置上游、隐藏真实 API Key、记录调用日志，并在企业内网中安全部署。

成功交付后，OpenHijack 将成为连接"固定客户端"与"开放上游生态"的关键基础设施，预期在个人用户市场降低模型切换门槛，在企业市场提供合规、可审计的代理层。

---

## Problem Statement

**Current Situation**
- Trae IDE 等客户端仅支持 OpenAI、OpenRouter 等固定服务商，无法直接配置私有/企业上游。
- 个人开发者需要手动修改 hosts、自签证书、搭建反向代理，门槛高且容易污染系统环境。
- 团队/企业缺乏统一的模型接入层，成员各自管理 API Key，存在泄露风险，且无法审计调用情况。

**Proposed Solution**
OpenHijack 提供一站式的本地 HTTPS 代理方案：
- 自动生成并安装自签名 CA，自动管理 hosts 劫持。
- 支持 OpenAI 兼容协议，并计划扩展 Anthropic、Gemini、OpenRouter 原生协议。
- 提供多配置组、热重载、模型映射、客户端鉴权、系统提示词覆盖等企业级能力。

**Business Impact**
- 个人用户：5 分钟内完成从安装到在 Trae 中调用自定义模型。
- 企业用户：统一密钥管理、配置隔离、审计日志，满足安全合规与成本核算需求。
- 生态影响：降低客户端与上游之间的耦合，促进多模型生态繁荣。

---

## Success Metrics

**Primary KPIs**
- **安装成功率**：新用户执行 `openhijack init` + `openhijack elevate` 后能在 Trae 中成功发出首个请求的比例 ≥ 90%。
- **跨平台稳定性**：Windows 10/11、macOS 12+、主流 Linux 发行版连续运行 7 天无崩溃、无 hosts/CA 残留错误的比例 ≥ 95%。
- **上游兼容性**：除 OpenAI 兼容接口外，支持至少 Anthropic、Gemini、OpenRouter 三种原生协议中的两种。
- **企业安全合规**：支持 API Key 集中管理、请求审计日志、配置组权限隔离，通过基础安全审计（无明文密钥落盘、日志可追溯）。
- **代理性能**：支持 100+ 并发连接，本地代理层 P95 延迟增加 < 20ms，具备限流与熔断机制。

**Validation**
- 安装成功率通过首次启动引导日志和错误上报统计。
- 跨平台稳定性通过 CI 自动化测试（多 OS 虚拟机）和用户反馈聚合。
- 上游兼容性通过集成测试覆盖各协议的标准请求/响应。
- 企业安全合规通过代码审计与单元测试验证。
- 代理性能通过负载测试与基准测试验证。

---

## User Personas

### Primary: 个人开发者 Alex
- **Role**：独立开发者 / 自由职业者
- **Goals**：在 Trae 中使用性价比更高或性能更强的自定义模型，随时切换不同上游。
- **Pain Points**：
  - Trae 只让选 OpenAI/OpenRouter，无法接入自己的 API Key 或私有模型。
  - 手动配证书、hosts 容易出错，搞坏系统网络配置后不会恢复。
  - 不想为了换模型而换 IDE。
- **Technical Level**：中级，熟悉命令行但不想深入研究 TLS/hosts 细节。

### Secondary: 企业管理员 Maya
- **Role**：某科技公司 AI 平台负责人
- **Goals**：为公司 50+ 名开发者统一配置模型上游，隐藏真实 API Key，审计调用成本。
- **Pain Points**：
  - 每个开发者都掌握真实 API Key，泄露风险高。
  - 不知道团队实际调用量、模型分布和成本。
  - 需要在内网环境部署，对安全和合规有要求。
- **Technical Level**：高级，熟悉企业网络、安全和运维流程。

---

## User Stories & Acceptance Criteria

### Story 1: 个人开发者一键接入自定义上游

**As a** 个人开发者 Alex
**I want to** 通过几条命令完成证书安装、hosts 劫持和代理启动
**So that** 我能在 Trae 中直接选择并使用自定义模型

**Acceptance Criteria**
- [ ] 运行 `openhijack init` 后，在默认配置目录生成可编辑的 `config.toml` 模板。
- [ ] 运行 `openhijack elevate` 后，自动完成权限提升、CA 安装、hosts 修改并启动代理。
- [ ] 在 Trae 中配置 `auth_key` 和 `mapped_model_id` 后，请求被成功转发到配置的上游。
- [ ] 停止代理后，hosts 条目和系统 CA 被自动清理（除非显式使用 `--no-manage`）。
- [ ] 若某一步失败，终端输出中文/英文错误提示和修复建议。

### Story 2: 多上游配置灵活切换

**As a** 个人开发者 Alex
**I want to** 在配置文件中定义多个上游并在运行时切换
**So that** 我能按项目或按模型快速切换不同服务商

**Acceptance Criteria**
- [ ] `config.toml` 支持 `[[config_groups]]` 定义多个上游配置组。
- [ ] 通过 `current_config_index` 指定默认启用的配置组。
- [ ] 支持配置热重载：修改配置文件后无需重启进程即可生效。
- [ ] 支持按模型名或路径规则把请求路由到不同配置组（Phase 2 增强）。

### Story 3: 企业统一密钥与审计

**As a** 企业管理员 Maya
**I want to** 集中管理上游 API Key 并记录请求日志
**So that** 我能在不暴露密钥的情况下为团队提供模型接入，并核算成本

**Acceptance Criteria**
- [ ] 支持配置组级别的密钥隔离，普通用户无法通过配置查看明文上游 API Key。
- [ ] 代理层记录每条请求的时间戳、模型 ID、上游配置组、状态码、Token 用量（如上游返回）、延迟。
- [ ] 日志支持输出到本地文件和可选的外部日志收集端（如 syslog / HTTP webhook）。
- [ ] 支持只读模式的管理员查看命令（如 `openhijack logs`），不会重启或中断代理。

### Story 4: 跨平台稳定运行与故障诊断

**As a** 用户 Alex / Maya
**I want to** 在 Windows、macOS、Linux 上都能稳定安装和运行
**So that** 我的团队或个人环境不受操作系统限制

**Acceptance Criteria**
- [ ] Windows 下 `openhijack elevate` 自动弹出 UAC 对话框，无需手动右键管理员运行。
- [ ] macOS / Linux 下自动调用 `sudo` 提升权限，失败时给出降级方案。
- [ ] 提供 `openhijack doctor` 命令检查证书、hosts、权限、上游连通性，并输出诊断报告。
- [ ] 卸载/清理命令能彻底移除所有修改过的系统配置和生成的证书文件。

---

## Functional Requirements

### Core Features

**Feature 1: 本地 HTTPS 代理与请求转发**
- **Description**：在本地 443 端口启动 HTTPS 服务器，拦截发往 `api.openai.com` 和 `openrouter.ai` 的请求，按配置转发到真实上游。
- **User Flow**：
  1. 用户运行 `openhijack elevate`。
  2. 系统生成/加载 CA 和服务器证书。
  3. 修改 hosts，将目标域名指向本地。
  4. 启动 HTTPS 代理，监听 443 端口。
  5. 客户端请求到达后，解析路径并转发到上游。
- **Edge Cases**：
  - 443 端口被占用：提示用户指定其他端口或使用 `--http` 模式。
  - 上游不可达：返回友好的 JSON 错误，包含上游地址和超时原因。
  - 客户端鉴权失败：返回 401，不向上游透传请求。
- **Error Handling**：所有网络错误均记录日志；向客户端返回 OpenAI 兼容的错误结构。

**Feature 2: 模型映射与 OpenAI 兼容 API**
- **Description**：对客户端暴露统一的 `mapped_model_id`，实际调用时映射到上游真实的 `model_id`。
- **User Flow**：
  1. 用户在 `config.toml` 中设置 `mapped_model_id = "my-model"`。
  2. 客户端请求 `/v1/models` 返回映射后的模型信息。
  3. 客户端请求 `/v1/chat/completions` 时，把请求体中的模型 ID 替换为实际上游模型 ID 后转发。
- **Edge Cases**：
  - 客户端传入未知模型 ID：返回 400 并提示可用的映射模型。
  - 上游返回的模型字段与客户端期望不一致：做字段归一化（如 `usage`、`choices` 结构）。
- **Error Handling**：映射失败时返回结构化错误，附带配置建议。

**Feature 3: 多配置组与热重载**
- **Description**：支持配置多个上游配置组，并在运行时热重载配置。
- **User Flow**：
  1. 用户在 `config.toml` 中定义多个 `[[config_groups]]`。
  2. 通过 `current_config_index` 选择默认配置组。
  3. 修改配置文件后，代理自动感知并应用新配置（不中断现有连接）。
- **Edge Cases**：
  - 热重载时配置文件格式错误：保持旧配置运行，并在日志中报警。
  - 配置组缺少必填字段（如 `api_url`）：启动时校验失败并给出明确错误。
- **Error Handling**：配置加载失败时拒绝启动，防止进入不可预期状态。

**Feature 4: 企业级安全与审计**
- **Description**：为企业提供密钥隔离、请求审计和访问控制能力。
- **User Flow**：
  1. 管理员在服务器/共享配置中写入上游 API Key。
  2. 普通用户通过客户端 `auth_key` 鉴权，但无法查看上游密钥。
  3. 代理记录请求日志，管理员可通过 `openhijack logs` 查看。
- **Edge Cases**：
  - 多个团队成员使用同一 `auth_key`：日志中记录该 Key 的标识以便追溯。
  - 上游密钥失效：记录错误码，并可选通知管理员。
- **Error Handling**：审计日志写入失败时，代理继续运行但发出警告，避免中断业务。

**Feature 5: 跨平台一键安装与诊断**
- **Description**：提供跨平台权限提升、证书/hosts 自动管理和故障诊断。
- **User Flow**：
  1. Windows 用户运行 `openhijack elevate`，自动触发 UAC。
  2. macOS/Linux 用户运行 `openhijack elevate`，自动调用 sudo。
  3. 使用 `openhijack doctor` 检查环境健康度。
- **Edge Cases**：
  - UAC/sudo 被拒绝：提示用户手动 sudo 或使用纯 HTTP 模式。
  - 系统缺少证书管理工具：提供安装命令或跳过 CA 安装（仅 HTTP 模式）。
- **Error Handling**：任何系统修改失败均回滚已修改部分，保持系统状态一致。

### Out of Scope
- 不提供云端 SaaS 服务或远程代理节点（本产品为本地/内网代理）。
- 不提供模型训练、微调或数据集管理功能。
- 不提供浏览器插件或 IDE 扩展（Phase 3 可探索）。
- 不提供计费与支付系统（企业可对接现有成本平台）。

---

## Technical Constraints

### Performance
- 支持 100+ 并发连接，CPU 占用在常规笔记本上 < 10%。
- 本地代理层 P95 延迟增加 < 20ms（不含上游响应时间）。
- SSE 流式响应延迟敏感，需使用低延迟转发，避免缓冲。
- 支持连接池复用，减少与上游建立 TLS 连接的开销。
- 具备基础限流（rate limiting）和熔断（circuit breaker）能力，防止上游过载。

### Security
- CA 私钥仅存于本地数据目录，具备合理文件权限（Unix 0600，Windows 仅当前用户可访问）。
- 上游 API Key 不暴露在客户端配置中，企业场景下建议与环境变量或密钥管理工具集成。
- 客户端 `auth_key` 支持可选启用，留空时不鉴权。
- 审计日志脱敏处理：不记录请求体中的敏感字段（如用户消息内容可配置是否记录）。
- 支持 `--disable-ssl-strict-mode` 用于内网测试，但默认启用上游 TLS 校验。
- 遵守 GDPR / 企业数据保护要求：日志保留策略可配置。

### Integration
- **Trae IDE**：核心目标客户端，通过默认 Base URL 被 hosts 劫持。
- **OpenAI 兼容上游**：已支持，继续作为第一优先级。
- **Anthropic / Gemini / OpenRouter 原生协议**：Phase 1 至少扩展其中两种。
- **日志收集**：可选对接 syslog、Fluentd、Loki、HTTP webhook。
- **密钥管理**：可选对接 HashiCorp Vault、AWS Secrets Manager、Azure Key Vault（Phase 2）。

### Technology Stack
- **语言**：Go 1.26+
- **配置解析**：`github.com/BurntSushi/toml`
- **Windows 系统调用**：`golang.org/x/sys/windows`
- **证书管理**：标准库 `crypto/x509`、`crypto/tls`
- **测试**：Go 标准测试 + `httptest`，关键路径覆盖 ≥ 80%。
- **CI/CD**：GitHub Actions，覆盖 Windows / macOS / Linux 构建与测试。
- **打包**：单二进制可执行文件，无额外运行时依赖。

---

## MVP Scope & Phasing

### Phase 1: MVP（初始发布）
- ✅ 跨平台一键安装、权限提升、CA/hosts 自动管理。
- ✅ OpenAI 兼容协议代理与模型映射。
- ✅ 多配置组配置与运行时切换（支持手动修改配置文件后热重载）。
- ✅ 客户端鉴权与系统提示词覆盖。
- ✅ 基础请求审计日志（本地文件）。
- ✅ `openhijack doctor` 故障诊断命令。
- ✅ 支持 Anthropic 或 Gemini 原生协议中的一种（达到"上游兼容性"最低目标）。

**MVP Definition**：个人开发者能在 5 分钟内完成安装并在 Trae 中调用自定义模型；小团队（5-20 人）能通过共享配置实现基础密钥隔离和日志审计。

### Phase 2: 增强版（发布后 3 个月）
- 🔹 支持 Anthropic、Gemini、OpenRouter 原生协议全部三种。
- 🔹 基于规则的多上游动态路由（按模型、用户、路径路由）。
- 🔹 企业级密钥管理集成（Vault、云 KMS）。
- 🔹 审计日志外发与可视化报表。
- 🔹 限流、熔断、缓存、配额管理。
- 🔹 Web GUI 管理界面（远期规划启动，提供核心页面与信息架构设计）。

### Phase 3: 生态扩展
- 🔮 IDE 插件 / 浏览器插件，实现配置一键同步。
- 🔮 多实例集群与高可用部署。
- 🔮 插件化上游协议扩展机制。
- 🔮 社区市场（预置配置模板、模型推荐）。

---

## Risk Assessment

| Risk | Probability | Impact | Mitigation Strategy |
|------|------------|--------|---------------------|
| 系统证书/hosts 修改失败或残留 | 中 | 高 | 所有修改均提供原子回滚；`cleanup`/`uninstall` 命令彻底清理；`doctor` 命令自检修复。 |
| 跨平台兼容性问题（UAC/sudo/证书工具差异） | 高 | 中 | 在 CI 中覆盖 Windows/macOS/Linux；提供各平台详细故障排查文档；支持纯 HTTP 降级模式。 |
| 上游协议变更导致映射失效 | 中 | 中 | 建立上游协议版本追踪与集成测试；关键字段归一化抽象层。 |
| 企业用户对安全合规要求超预期 | 中 | 高 | Phase 1 即纳入密钥隔离、审计日志；明确 Phase 2 引入 KMS 集成与更细粒度权限。 |
| 客户端更新绕过 hosts 劫持（如硬编码 IP） | 低 | 高 | 持续关注目标客户端更新；提供 PAC/系统代理等备选劫持方案。 |
| 高并发下本地 443 端口或资源瓶颈 | 中 | 中 | 性能基准测试；支持端口自定义与多实例部署；连接池与限流机制。 |

---

## Dependencies & Blockers

**Dependencies**
- **Go 1.26+ 构建环境**：已具备，持续维护版本兼容性。
- **`github.com/BurntSushi/toml`**：已引入，用于配置文件解析。
- **目标客户端（Trae）网络行为稳定**：需跟踪 Trae 更新是否改变 Base URL 或证书校验逻辑。
- **企业 KMS/日志平台**（Phase 2）：可选依赖，不影响 MVP。

**Known Blockers**
- 暂无。Phase 1 所有功能均可基于现有架构独立完成。

---

## Appendix

### Glossary
- **CA（Certificate Authority）**：证书颁发机构，此处指 OpenHijack 自签名根证书。
- **Hosts 劫持**：修改系统 hosts 文件，使特定域名解析到本地地址。
- **上游（Upstream）**：实际提供模型 API 服务的服务商或网关。
- **配置组（Config Group）**：`config.toml` 中定义的一组上游连接配置。
- **模型映射**：客户端看到的模型 ID 与实际上游模型 ID 之间的转换。
- **SSE（Server-Sent Events）**：服务器推送事件，常用于流式模型响应。

### References
- [README.md](/README.md)：项目简介与快速开始。
- [Product-Spec.md](/Product-Spec.md)：现有产品规格文档。
- [API-Key-Security-Storage-Design.md](/docs/API-Key-Security-Storage-Design.md)：API Key 安全存储设计。
- 上游协议文档：
  - OpenAI API: https://platform.openai.com/docs/api-reference
  - Anthropic Messages API: https://docs.anthropic.com/en/api/messages
  - Gemini API: https://ai.google.dev/api
  - OpenRouter API: https://openrouter.ai/docs

---

*This PRD was created through interactive requirements gathering with quality scoring to ensure comprehensive coverage of business, functional, UX, and technical dimensions.*
