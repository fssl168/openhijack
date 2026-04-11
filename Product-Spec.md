# 产品需求文档 (Product Specification)

## 项目概述

**项目名称**：OpenHijack 多操作系统支持改造

**一句话描述**：将 OpenHijack 从 Linux/macOS 专用工具改造为支持 Windows、macOS、Linux 三大操作系统的跨平台 HTTPS 代理工具

**目标**：消除平台依赖，使 OpenHijack 能在 Windows 系统上完整运行，包括 hosts 文件修改、权限提升、CA 证书安装等核心功能

## 目标用户

**核心用户群体**：需要在 Windows/macOS/Linux 上使用 OpenAI 兼容 API 代理的开发者

**用户特征**：
- 年龄段：20-45岁
- 技术水平：中高级开发者，熟悉命令行操作
- 使用场景：
  - Windows 用户使用 Trae IDE 等 OpenAI 客户端
  - macOS 开发者进行跨平台开发
  - Linux 服务器部署代理服务
- 痛点：
  - 当前版本无法在 Windows 上运行（hosts文件路径错误、缺少sudo）
  - 需要手动配置证书和hosts，门槛高
  - 跨平台迁移困难

## 核心功能

### 功能 1：跨平台 Hosts 文件管理

**功能描述**：自动检测操作系统并使用正确的 hosts 文件路径，支持添加/删除/备份/恢复操作

**输入**：
- 操作系统类型：runtime.GOOS (linux/darwin/windows)
- 域名映射：api.openai.com -> 127.0.0.1 / ::1
- 数据目录：用于存储备份文件

**输出**：
- 成功：hosts 文件已更新，日志记录操作详情
- 失败：返回具体错误信息（权限不足、文件锁定等）

**业务规则**：
- **Windows**：`C:\Windows\System32\drivers\etc\hosts`
- **Linux/macOS**：`/etc/hosts`
- 备份文件统一存储在 dataDir/hosts.backup
- 添加条目前必须先备份原始文件
- 使用标记注释 `# Added by OpenHijack` 标识管理的条目

**异常处理**：
- 权限不足（Windows需要管理员权限）：提示用户以管理员身份运行
- 文件被占用：重试3次或提示关闭占用程序
- 备份文件不存在：跳过恢复并记录日志

**优先级**：高

**状态**：待开发

---

### 功能 2：跨平台权限提升机制

**功能描述**：替换 Unix-only 的 sudo 机制，实现跨平台的权限提升方案

**输入**：
- 当前权限级别：os.Geteuid() 或等效检测
- 要执行的命令和参数
- 环境变量保留列表

**输出**：
- 成功：以提升的权限执行目标命令
- 失败：返回权限提升失败原因

**业务规则**：
- **Linux/macOS**：保持现有 sudo 机制
  - 使用 `exec.Command("sudo", args...)`
  - 通过 `--preserve-env` 保留环境变量
  - 使用 `syscall.Exec` 替换当前进程
- **Windows**：实现自动 UAC 提升方案（方案D - 推荐）
  - **首选方案**：使用 `ShellExecuteEx` + `"runas"` 动词自动触发 UAC 提示
  - **实现步骤**：
    1. 调用 `golang.org/x/sys/windows.IsUserAnAdmin()` 检测当前权限
    2. 如果未提升，调用 `windows.ShellExecuteEx()` 以管理员身份重新启动自身
    3. 传递所有命令行参数给新进程（包括 --config、环境变量等）
    4. 当前非特权进程优雅退出（os.Exit(0)）
  - **技术依赖**：`golang.org/x/sys/windows`（使用 build tags 仅在 Windows 编译）
  - **参考实现**：gsudo (https://github.com/gerardog/gsudo) - 4000+ stars, 700k+ 下载量
  - **降级方案**：
    - 如果 UAC 失败（用户拒绝/UAC禁用），提供详细的错误提示
    - 建议用户手动以管理员身份运行或使用 `--http --port 8787` 纯HTTP模式
- 所有平台：提供 `--no-manage` 选项跳过权限相关操作（纯HTTP模式）

**异常处理**：
- **Linux/macOS**：
  - sudo 密码认证失败：显示错误信息并退出
  - sudo 未安装：提示用户安装 sudo 或使用 root 账户运行
- **Windows**：
  - UAC 提示被用户拒绝：优雅退出并给出降级方案（--http 模式或手动提权）
  - UAC 已被系统管理员禁用：提示联系 IT 部门或手动以管理员身份运行
  - ShellExecuteEx 调用失败：记录详细错误码（Windows 错误码参考表）
  - 无限循环防护：确保提权后的进程不会再尝试提权（通过 IsPrivileged() 检查）
- **通用**：
  - 无法检测权限级别：假设无特权，尝试执行并在失败时提示
  - 参数传递安全：使用 ShellExecuteEx 而非 exec.Command，避免命令注入

**优先级**：高

**状态**：待开发

---

### 功能 3：跨平台路径和目录管理

**功能描述**：遵循各操作系统标准规范管理配置文件和数据文件路径

**输入**：
- 操作系统类型：runtime.GOOS
- 用户主目录：通过 os.UserHomeDir() 获取

**输出**：
- 配置目录路径
- 数据目录路径
- 各类文件的完整路径

**业务规则**：

**配置文件路径**（config.toml）：
| 操作系统 | 路径 |
|---------|------|
| Linux | `$HOME/.config/openhijack/config.toml` (XDG标准) |
| macOS | `$HOME/.config/openhijack/config.toml` (或 `$HOME/Library/Application Support/openhijack/`) |
| Windows | `%APPDATA%\openhijack\config.toml` |

**数据文件路径**（CA证书、hosts备份等）：
| 操作系统 | 路径 |
|---------|------|
| Linux | `$HOME/.local/share/openhijack/` (XDG标准) |
| macOS | `$HOME/.local/share/openhijack/` (或 `$HOME/Library/Application Support/openhijack/`) |
| Windows | `%LOCALAPPDATA%\openhijack\` |

**特殊路径处理**：
- **root/administrator 主目录**：
  - Linux: `/root`
  - Windows: `C:\Users\Administrator` (通过 os.UserHomeDir() 自动获取)
  - 不再硬编码 `/root/.config/openhijack`

**异常处理**：
- 无法获取用户主目录：使用当前工作目录作为后备
- 目录创建失败：检查父目录权限，给出明确错误信息

**优先级**：高

**状态**：待开发

---

### 功能 4：跨平台系统调用兼容层

**功能描述**：封装平台特定的系统调用，提供统一的接口

**输入**：
- 操作上下文（当前平台）
- 操作类型（权限检测、进程替换等）

**输出**：
- 平台无关的操作结果

**业务规则**：

**权限检测**：
```go
// 伪代码
func isPrivileged() bool {
    switch runtime.GOOS {
    case "linux", "darwin":
        return os.Geteuid() == 0
    case "windows":
        // Windows: 检查当前进程是否以管理员身份运行
        // 使用 syscall.Token().IsElevated() 或 golang.org/x/sys/windows
        return windows.IsElevated()
    }
}
```

**进程替换**：
- Linux/macOS: `syscall.Exec(bin, args, env)`
- Windows: 不支持 Exec，改用 `exec.Command` 启动子进程并等待完成

**文件权限**：
- Linux/macOS: 保持 Unix 权限模式 (0700, 0600, 0644)
- Windows: 忽略权限位（NTFS ACL 机制不同），但保持代码兼容性

**异常处理**：
- 系统调用不支持：返回 "not implemented on this platform" 错误
- 权限检测失败：默认为非特权模式

**优先级**：高

**状态**：待开发

---

### 功能 5：增强的 CA 证书多平台支持

**功能描述**：在现有 CA 证书安装基础上，增加更多 Linux 发行版支持和错误恢复机制

**输入**：
- CA 证书文件路径
- 日志回调函数

**输出**：
- 安装成功/失败状态
- 详细的日志信息

**业务规则**：

**已有支持（保持不变）**：
- ✅ Arch Linux (trust command)
- ✅ RHEL/Fedora/CentOS (update-ca-trust)
- ✅ Debian/Ubuntu (update-ca-certificates)
- ✅ macOS (security command)
- ✅ Windows (certutil)

**新增支持**：
- 🆕 **openSUSE**: 使用 `update-ca-certificates` (与Debian相同)
- 🆕 **Alpine Linux**: 使用 `ca-certificates -fakera` 或手动复制到 `/usr/local/share/ca-certificates/`
- 🆕 **FreeBSD**: 使用 `certctl rehash` 和 `/usr/local/share/certs/`

**改进点**：
- 安装失败时提供手动安装指引
- 卸载时清理所有可能的安装位置
- 增加证书指纹验证（防止中间人攻击）

**异常处理**：
- 找不到证书工具：列出该发行版需要的包名（apt/yum/pacman install xxx）
- 权限不足：明确提示需要 root 权限
- 证书已存在：跳过安装并记录日志

**优先级**：中

**状态**：待开发

---

### 功能 6：Windows 特殊适配功能

**功能描述**：解决 Windows 平台特有的兼容性问题

**输入**：
- Windows 系统环境
- 应用程序运行上下文

**输出**：
- 适配后的行为

**业务规则**：

**端口占用处理**：
- Windows 上 443 端口可能被 IIS/Hyper-V/System Reserve 占用
- 自动检测端口占用情况，建议替代端口（8443, 9443）
- 提供 `--port` 参数允许自定义端口

**防火墙规则**：
- 首次运行时可选添加 Windows Defender 防火墙入站规则
- 规则名称："OpenHijack Proxy (Port {port})"
- cleanup 时自动移除防火墙规则

**服务注册（可选）**：
- 支持将 openhijack 注册为 Windows Service（使用 golang.org/x/sys/svc）
- 允许开机自启动
- 通过 `openhijack service install/uninstall/start/stop` 管理

**路径长度限制**：
- Windows MAX_PATH = 260 字符
- 使用 `\\?\` 前缀支持长路径（Go 1.26+ 已内置支持）

**控制台编码**：
- Windows 控制台默认使用 GBK/CP936 编码
- 启动时设置 UTF-8 编码：`os.Setenv("PYTHONUTF8", "1")` 对Go无效
- Go 1.26+ 应自动处理，否则需显式设置控制台代码页

**异常处理**：
- 端口占用：提示用户释放端口或使用其他端口
- 防火墙添加失败：记录警告但不阻塞主流程
- 服务注册失败：回退到前台模式

**优先级**：中

**状态**：待开发

---

## 功能优先级

### 高优先级（必须有）- 阻塞Windows基本使用
1. **跨平台 Hosts 文件管理** - 无此功能Windows完全无法劫持域名
2. **跨平台权限提升机制** - 无此功能无法在Windows上获得管理员权限
3. **跨平台路径和目录管理** - 无此功能配置文件存储位置错误
4. **跨平台系统调用兼容层** - 无此功能程序直接崩溃或编译失败

### 中优先级（应该有）- 提升用户体验
5. **增强的 CA 证书多平台支持** - 支持更多边缘发行版
6. **Windows 特殊适配功能** - 解决端口占用、防火墙等问题

### 低优先级（可以有）- 锦上添花
7. **自动化测试覆盖** - 为每个平台编写集成测试
8. **文档完善** - 更新README增加Windows使用说明
9. **CI/CD 多平台构建** - GitHub Actions 构建 Windows/macOS/Linux 二进制

## AI增强功能

本项目暂不需要 AI 增强，因为核心问题是平台兼容性技术问题，不涉及智能决策或自然语言处理。

## 非功能性需求

### 性能要求
- 启动时间：< 2秒（含证书检查和hosts修改）
- 代理转发延迟：< 10ms（不含上游响应时间）
- 内存占用：< 50MB（空闲状态）

### 安全要求
- CA 私钥权限：仅所有者可读写（Unix 0600 / Windows ACL）
- 配置文件权限：仅所有者可读写（Unix 0600 / Windows ACL）
- Hosts 修改：必须备份原文件
- 通信加密：强制 TLS（除非显式 --http 模式）
- 敏感信息：不在日志中打印 API Key

### 可用性要求
- 错误信息清晰：包含问题描述 + 解决建议
- 跨平台一致性：相同功能在不同平台表现一致
- 向后兼容：不破坏现有 Linux/macOS 用户的使用方式
- 降级方案：--http --no-manage 模式无需任何特权

### 兼容性要求
- **操作系统**：
  - ✅ Windows 10/11 (64-bit)
  - ✅ Windows Server 2019/2022
  - ✅ macOS 12+ (Intel & Apple Silicon)
  - ✅ Ubuntu 20.04/22.04/24.04
  - ✅ Debian 11/12
  - ✅ Fedora 39/40+
  - ✅ Arch Linux (rolling)
  - ✅ RHEL 8/9
  - ✅ Alpine Linux 3.19+
  - ✅ openSUSE Tumbleweed
- **Go 版本**：1.26+（已满足）
- **架构**：amd64, arm64, armv7l (Raspberry Pi)

## 技术栈建议

### 当前技术栈（保持不变）
- **语言**：Go 1.26+
- **配置解析**：github.com/BurntSushi/toml
- **HTTP 代理**：net/http 标准库
- **TLS 证书**：crypto/tls, crypto/x509, crypto/rsa 标准库

### 新增依赖（按需引入）
- **Windows API**：golang.org/x/sys/windows（权限检测、服务管理）
- **跨平台路径**：os.UserHomeDir()（标准库，已使用）
- **条件编译**：Go build tags（//go:build linux, //go:build windows 等）

### 架构设计
```
internal/
├── platform/
│   ├── platform.go          # 平台检测接口定义
│   ├── linux.go             # Linux 实现
│   ├── darwin.go            # macOS 实现
│   └── windows.go           # Windows 实现
├── cert/
│   └── install.go           # 已有多平台支持（增强）
├── hosts/
│   └── hosts.go             # 改造为跨平台
├── config/
│   └── config.go            # 路径逻辑抽取到 platform
└── proxy/
    └── ...                  # 无需改动（纯网络逻辑）
```

## 附录

### 术语表
- **UAC (User Account Control)**：Windows 用户账户控制，用于权限提升确认
- **XDG Base Directory**：Linux 桌面组标准化目录规范（~/.config, ~/.local/share）
- **CA (Certificate Authority)**：证书颁发机构，用于签发TLS证书
- **Hosts 劫持**：通过修改 hosts 文件将域名指向本地地址
- **SSE (Server-Sent Events)**：服务器推送事件流，用于 ChatGPT 流式响应
- **sudo (Superuser DO)**：Unix 系统以超级用户身份执行命令的工具
- **certutil**：Windows 证书管理工具
- **security**：macOS 钥匙串和证书管理命令行工具

### 参考资料
- [Go build constraints](https://go.dev/doc/go/build#constraints)
- [golang.org/x/sys/windows](https://pkg.go.dev/golang.org/x/sys/windows)
- [Windows UAC documentation](https://docs.microsoft.com/en-us/windows/security/identity-protection/user-account-control/how-it-works)
- [XDG Base Directory specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html)
- [Windows certificate store](https://docs.microsoft.com/en-us/windows-hardware/drivers/install/certificate-stores)
- [Go os package cross-platform](https://pkg.dev/os/#pkg-variables)

### 改造影响范围评估

#### 必须修改的文件
1. `cmd/openhijack/main.go`
   - 删除所有 `os.Geteuid()` 调用 → 使用 platform 包
   - 删除 `exec.Command("sudo", ...)` → 使用 platform.Elevate()
   - 删除 `syscall.Exec()` → 使用 platform.ExecReplace()
   - 修改 `getConfigDir()/getDataDir()` → 使用 platform.GetConfigDir()/GetDataDir()
   - 修改硬编码的 `/root/.config/openhijack` → platform.GetAdminConfigDir()

2. `internal/hosts/hosts.go`
   - 将 `const HostsFile = "/etc/hosts"` 改为动态获取
   - 新增 `GetHostsPath() string` 函数（从 platform 包获取）

#### 需要增强的文件
3. `internal/cert/install.go`
   - 增加 Alpine Linux 支持
   - 增加 openSUSE 支持
   - 增加 FreeBSD 支持
   - 增加更详细的错误提示和手动安装指引

#### 需要新建的文件
4. `internal/platform/platform.go` - 接口定义和平台检测
5. `internal/platform/linux.go` - Linux 实现
6. `internal/platform/darwin.go` - macOS 实现
7. `internal/platform/windows.go` - Windows 实现

#### 可能需要微调的文件
8. `internal/proxy/*.go` - 通常不需要改动（纯网络逻辑），但需验证是否有隐式平台依赖

### 测试策略

#### 单元测试（每个函数）
- platform.IsPrivileged() - mock 不同平台
- platform.GetHostsPath() - 验证路径正确性
- platform.GetConfigDir() - 验证 XDG/AppData 路径
- hosts.AddEntry/RemoveEntry - 使用临时文件测试

#### 集成测试（关键流程）
- 完整 elevate 流程（需要实际权限，标记为 +build integration）
- CA 证书安装/卸载循环
- Hosts 修改/恢复循环

#### 手动测试矩阵
| 操作系统 | elevate | serve (TLS) | serve (HTTP) | cleanup | init |
|---------|---------|-------------|--------------|---------|------|
| Ubuntu 22.04 | ✅ | ✅ | ✅ | ✅ | ✅ |
| Windows 11 | ✅ | ✅ | ✅ | ✅ | ✅ |
| macOS 14 | ✅ | ✅ | ✅ | ✅ | ✅ |

### 风险评估

#### 高风险项
1. **Windows 权限提升** - UAC 机制复杂，可能需要多次迭代
2. **Windows Hosts 修改** - 可能触发杀毒软件告警
3. **回归风险** - 修改可能破坏现有 Linux/macOS 功能

#### 缓解措施
1. **充分测试**：在 CI 中加入多平台测试
2. **渐进式重构**：先提取接口，再逐个替换实现
3. **特性开关**：使用 build tag 隔离平台代码，互不影响
4. **回滚计划**：保留原有代码作为 fallback

---

**文档版本**：1.0

**最后更新**：2026-04-11

**下次更新计划**：完成首个平台（推荐 Windows）的实现后更新进度
