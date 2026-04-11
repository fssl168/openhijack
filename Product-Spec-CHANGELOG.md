# 产品变更记录 (Changelog)

本文档记录产品需求文档的所有变更历史，遵循语义化版本规范（Semantic Versioning）。

---

## [1.0.0] - 2026-04-11

### 变更类型：新增

### 新增功能
- **跨平台 Hosts 文件管理**：支持 Windows (`C:\Windows\System32\drivers\etc\hosts`)、Linux/macOS (`/etc/hosts`) 的自动检测和管理
- **跨平台权限提升机制**：替换 Unix-only 的 sudo，实现 Linux/macOS (sudo) + Windows (UAC/管理员检测) 双轨制
- **跨平台路径和目录管理**：遵循 XDG 标准（Linux）、AppData（Windows）规范存储配置和数据文件
- **跨平台系统调用兼容层**：封装 platform 包，提供 IsPrivileged()、Elevate()、ExecReplace() 等统一接口
- **增强的 CA 证书多平台支持**：新增 Alpine Linux、openSUSE、FreeBSD 发行版支持
- **Windows 特殊适配功能**：端口占用检测、防火墙规则管理、服务注册（可选）

### 修改功能
- **Hosts 文件路径**：从硬编码 `/etc/hosts` 改为动态获取 `platform.GetHostsPath()`
- **权限提升**：从直接调用 `exec.Command("sudo", ...)` 改为 `platform.Elevate()`
- **进程替换**：从 `syscall.Exec()` 改为 `platform.ExecReplace()`（Windows 使用子进程）
- **配置/数据目录**：从硬编码 XDG 路径改为 `platform.GetConfigDir()/GetDataDir()`
- **root 主目录**：从硬编码 `/root` 改为动态获取管理员主目录
- **Windows 权限提升方案优化**（2026-04-11）：
  - **原方案C（已废弃）**：检测是否管理员，非管理员时提示用户手动右键"以管理员身份运行"
    - 问题：用户体验差（需5-6步手动操作）、不符合行业惯例、脚本不兼容
  - **新方案D（已采用）**：使用 ShellExecuteEx + "runas" 动词自动触发 UAC 提示
    - 优势：一键完成、符合主流工具做法（gsudo/Docker/Microsoft sudo）、完全自动化
    - 技术实现：调用 golang.org/x/sys/windows.IsUserAnAdmin() 检测权限，未提升时自动重启自身并触发UAC
    - 参考实现：gsudo (4000+ stars, 700k+ downloads)
    - 降级策略：UAC失败时提供详细错误提示和 --http 模式替代方案

### 删除功能
- 无

### 功能调整
- 所有功能的优先级均为"高"，因为缺少任一功能都会导致 Windows 无法使用

### AI增强调整
- 无 AI 增强功能（纯技术兼容性问题）

### 非功能性需求调整
- **兼容性要求**：从仅支持 Linux/macOS 扩展到支持 Windows 10/11、macOS 12+、主流 Linux 发行版
- **性能要求**：保持不变（启动 <2秒，延迟 <10ms）
- **安全要求**：增加 Windows ACL 权限控制说明

### 技术栈调整
- **新增依赖**：golang.org/x/sys/windows（Windows API 支持）
- **架构重构**：新建 internal/platform/ 包，使用 Go build tags 实现条件编译

### 影响范围
- **cmd/openhijack/main.go**（高影响）：核心逻辑全面改造，删除所有 Unix-only 调用
- **internal/hosts/hosts.go**（高影响）：HostsFile 常量改为动态函数
- **internal/cert/install.go**（中影响）：增加新发行版支持代码
- **internal/proxy/**（低影响）：预计无需修改，需验证无隐式平台依赖

### 兼容性说明
- **向后兼容**：✅ 是的，现有 Linux/macOS 用户无需任何改动
- **迁移指南**：
  - 现有用户可直接升级，行为完全一致
  - 配置文件格式不变（config.toml）
  - 数据文件位置不变（~/.local/share/openhijack/）
  - 命令行参数完全兼容
- **注意事项**：
  - 首次在 Windows 上运行需要以管理员身份执行 elevate 或 cleanup
  - Windows 用户建议使用 `--http --port 8787` 进行初始测试（避免 443 端口权限问题）

---

## 版本号规范

遵循语义化版本规范：`MAJOR.MINOR.PATCH`

- **MAJOR（主版本号）**：不兼容的 API 修改或重大功能变更
- **MINOR（次版本号）**：向下兼容的功能性新增
- **PATCH（修订号）**：向下兼容的问题修正

示例：
- `1.0.0` - 初始版本（本次多OS支持改造）
- `1.1.0` - 新增功能，向后兼容
- `1.1.1` - 修复 bug，向后兼容
- `2.0.0` - 重大变更，不向后兼容

## 变更类型说明

### 新增（Added）
- 新增功能
- 新增 AI 能力
- 新增技术栈组件

### 修改（Changed）
- 修改现有功能
- 调整业务规则
- 优化用户体验

### 删除（Removed）
- 删除废弃功能
- 移除 AI 能力
- 移除技术栈组件

### 重构（Refactored）
- 代码重构
- 架构调整
- 性能优化

## 记录规范

### 每次更新必须包含：
1. ✅ 版本号（遵循语义化版本规范）
2. ✅ 变更日期（YYYY-MM-DD）
3. ✅ 变更类型（新增/修改/删除/重构）
4. ✅ 详细变更内容（列出所有变更项）
5. ✅ 影响范围（说明哪些功能受影响）
6. ✅ 兼容性说明（是否向后兼容）

### 变更描述要求：
- 清晰、具体、无歧义
- 使用"用户"而非"我"（避免主观表述）
- 说明"为什么变更"而非"变更了什么"
- 对 AI 增强功能，说明 AI 能力的具体变化

### 影响范围说明：
- 列出所有受影响的功能
- 说明影响程度（高/中/低）
- 提供迁移建议（如果不兼容）

---

**文档版本**：1.0.0

**最后更新**：2026-04-11

**下次更新计划**：完成首个平台实现后更新（推荐先实现 Windows 支持）
