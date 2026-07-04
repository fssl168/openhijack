# OpenHijack 全栈对齐计划

**生成时间**：2026-07-04
**更新时间**：2026-07-04
**范围**：后端业务管线集成 + 前端交互流程对齐
**基准**：实地代码探索

---

## 1. 执行摘要

### 1.1 实施状态：✅ 已全部完成

| 阶段 | 任务 | 状态 | 验证 |
|------|------|------|------|
| A1 | AuditLogger 集成到 ProxyServer | ✅ 已完成 | `go test ./internal/audit/...` 通过 |
| A2 | Config Watcher 集成到 ProxyServer | ✅ 已完成 | 运行时配置热重载 |
| A3 | 认证机制增强 | ✅ 已完成 | `auth.go` 使用 ConstantTimeCompare + RateLimiter |
| B1 | Doctor Wails 绑定 | ✅ 已完成 | `gui/app.go` 暴露 RunDoctor/GetLastDoctorResults |
| B2 | AuditLog Wails 绑定 | ✅ 已完成 | `gui/app.go` 暴露 GetAuditLogs/GetAuditLogPath/ClearAuditLogs |
| B3 | Watcher Wails 绑定 | ✅ 已完成 | `gui/app.go` 暴露 GetWatcherStatus/ReloadConfigManually |
| C1 | runtime.ts 绑定扩展 | ✅ 已完成 | 添加 8 个新绑定 + 事件订阅 |
| C2 | types/index.ts 扩展 | ✅ 已完成 | 添加 DoctorResult/AuditEntry/WatcherStatus |
| C3 | services/index.ts 扩展 | ✅ 已完成 | 添加 DoctorService/AuditService/WatcherService |
| C4 | UI Store 扩展 | ✅ 已完成 | 添加 doctor/audit 视图类型 |
| D1 | Doctor 健康检查视图 | ✅ 已完成 | `views/Doctor.vue` |
| D2 | AuditLog 审计日志视图 | ✅ 已完成 | `views/AuditLog.vue` |
| D3 | App.vue 导航扩展 | ✅ 已完成 | 添加 doctor/audit 导航项 |
| D4 | 配置重载通知流程 | ✅ 已完成 | WatcherService.onConfigReloaded 事件订阅 |

---

## 2. 修复记录

### 2.1 Crypto 加密往返 Bug 修复（2026-07-04）

在实施对齐过程中发现并修复了报告之外的真实 bug：

| # | 文件 | 问题 | 修复 |
|---|------|------|------|
| 1 | [crypto.go](file:///home/wolf/.openclaw/workspace/openhijack/internal/crypto/crypto.go#L166) | `parseEncryptedString` 使用 `fmt.Sscanf("%s:%s:...")` 导致 n=1 而非 7，Encrypt/Decrypt 往返实际是坏的 | 改用 `strings.SplitN` 切分 7 段 |
| 2 | [crypto.go](file:///home/wolf/.openclaw/workspace/openhijack/internal/crypto/crypto.go#L162) | `IsEncrypted` 使用 `len > 4` 导致 `IsEncrypted("enc:")` = false | 改用 `strings.HasPrefix` |
| 3 | [crypto.go](file:///home/wolf/.openclaw/workspace/openhijack/internal/crypto/crypto.go#L203) | `GenerateMasterPassword` 生成的密码有时含 "fedc"/"1234" 连续模式被验证拒绝 | 添加重试循环 |
| 4 | [crypto_test.go](file:///home/wolf/.openclaw/workspace/openhijack/internal/crypto/crypto_test.go#L89) | `TestValidateMasterPassword` 4 个用例描述与实际密码内容不符 | 重写 11 个测试用例 |

**验证结果**：
```
go test ./internal/crypto/...               ✅ ok (33.18s)
go test ./internal/proxy/...                ✅ ok
go test ./internal/audit/...                ✅ ok
go test ./internal/health/...               ✅ ok
go test ./internal/config/...               ✅ ok
go build ./...                              ✅ exit 0
vue-tsc --noEmit                            ✅ exit 0
```

---

## 3. 新增文件清单

### 后端 (Go)

| 文件 | 用途 |
|------|------|
| `internal/audit/audit.go` | 审计日志记录器 |
| `internal/config/watcher.go` | 配置文件监听与热重载 |
| `internal/health/checks.go` | 共享健康检查逻辑 |
| `internal/health/checks_test.go` | 健康检查测试 |
| `cmd/openhijack/doctor.go` | CLI doctor 命令（重构为调用 internal/health） |
| `internal/proxy/audit_watcher_test.go` | Audit + Watcher 集成测试 |
| `gui/app_bindings_test.go` | Wails 绑定测试 |

### 前端 (Vue/TypeScript)

| 文件 | 用途 |
|------|------|
| `gui/frontend/src/views/Doctor.vue` | 健康检查视图 |
| `gui/frontend/src/views/AuditLog.vue` | 审计日志视图 |
| `gui/frontend/src/composables/useSharedState.ts` | 共享状态工具（组合式 Store） |
| `gui/frontend/src/stores/types.ts` | Store 类型定义 |
| `gui/frontend/src/stores/index.ts` | 组合式 Store 入口 |
| `gui/frontend/src/utils/aria.ts` | ARIA 工具库 |

### 文档

| 文件 | 用途 |
|------|------|
| `.trae/documents/OpenHijack-Final-Alignment-Report.md` | 最终对齐度报告 |

---

## 4. 验证步骤

### 4.1 后端验证

```bash
# 1. 编译验证
go build ./...

# 2. 单元测试
go test ./internal/crypto/... -v
go test ./internal/proxy/... -v -cover
go test ./internal/audit/... -v -cover
go test ./internal/config/... -v -cover
go test ./internal/health/... -v -cover

# 3. GUI 绑定测试
cd gui && go test -v -cover
```

### 4.2 前端验证

```bash
cd gui/frontend
export PATH="/home/wolf/.nvm/versions/node/v22.22.2/bin:$PATH"
./node_modules/.bin/vue-tsc --noEmit
npm run build
```

### 4.3 手动验证清单

- [x] 首次启动显示 Onboarding Wizard
- [x] 创建配置后启动代理
- [x] 发起 API 请求，在"审计日志"视图看到记录
- [x] 修改配置文件，收到"配置已热重载"通知
- [x] 在"健康检查"视图点击"重新检查"，看到检查结果
- [x] 在"设置"视图看到 Watcher 状态和"手动重载"按钮

---

## 5. 已知限制

1. **Crypto 测试**：部分预存测试用例在历史版本中描述与实际不符，已在 2026-07-04 修复并更新测试用例
2. **API Key 加密存储**：仍使用默认主密钥，生产环境建议配置自定义密钥
3. **Audit log 文件大小**：未实现 logrotate，长时间运行需手动清理

---

**计划文件路径**：`/home/wolf/.openclaw/workspace/openhijack/.trae/documents/openhijack-fullstack-alignment-plan.md`