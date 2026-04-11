# OpenHijack

本地 HTTPS 代理服务器，用于让仅支持固定服务商（如 OpenAI、OpenRouter）的客户端（如 Trae IDE）通过本地代理实际使用自选上游服务。

**✨ 多平台支持：Windows 10/11、macOS 12+、Linux（Debian/Ubuntu/RHEL/Fedora/Arch/Alpine/openSUSE）、FreeBSD**

## 工作原理

```
客户端 (Trae) → https://api.openai.com:443 或 https://openrouter.ai:443
                        ↓ (hosts 劫持)
              127.0.0.1:443 (本地 HTTPS 代理)
                        ↓ (转发到真实上游)
              https://your-upstream-provider/v1/...
```

1. 生成自签名 CA 证书（CN=OpenHijack_CA），签发 `api.openai.com` 和 `openrouter.ai` 服务器证书
2. 安装 CA 到系统信任库（自动检测平台并调用对应工具）
3. 修改系统 hosts 文件，将域名指向 `127.0.0.1` / `::1`：
   - **Linux/macOS**: `/etc/hosts`
   - **Windows**: `C:\Windows\System32\drivers\etc\hosts`
4. 在本地 443 端口启动 HTTPS 服务器
5. 拦截所有发往目标域名的请求，转发到配置的上游地址：
   - `GET /v1/models` — 返回映射后的模型信息
   - `POST /v1/chat/completions` — 转发到上游（支持 SSE 流式/非流式）
   - 其他路径 — 直接透传到上游
6. 停止时自动清理 hosts 条目和系统 CA

## 功能

- ✅ **多操作系统支持**：Windows（自动 UAC 提权）、macOS、Linux、FreeBSD
- ✅ OpenAI 兼容 API 代理（模型 ID 映射）
- ✅ **多域名劫持**：支持 `api.openai.com` 和 `openrouter.ai`
- ✅ SSE 流式 / 非流式响应透传
- ✅ 客户端鉴权
- ✅ 系统提示词捕获与覆盖
- ✅ 自动 CA 证书生成与系统安装
- ✅ 自动 hosts 文件管理（跨平台路径）
- ✅ 权限提升：
  - Linux/macOS: sudo
  - Windows: 自动 UAC 提示（无需手动右键）
- ✅ 多平台 CA 安装：
  - Arch Linux (`trust`)
  - RHEL/CentOS/Fedora/openSUSE (`update-ca-trust`)
  - Debian/Ubuntu/Alpine (`update-ca-certificates`)
  - macOS (`security`)
  - Windows (`certutil`)
  - FreeBSD (`certctl`)

## 构建

### 单平台构建

```bash
go build -ldflags="-s -w" -o openhijack ./cmd/openhijack/
```

### 多平台交叉编译

```bash
# Windows 64位
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o openhijack-windows-amd64.exe ./cmd/openhijack/

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o openhijack-macos-arm64 ./cmd/openhijack/

# Linux 64位
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o openhijack-linux-amd64 ./cmd/openhijack/

# Linux ARM (树莓派)
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o openhijack-linux-arm64 ./cmd/openhijack/
```

## 使用

### 第一步：初始化配置

```bash
./openhijack init
```

会在默认配置路径生成 `config.toml` 模板，并提示你修改上游配置。

**配置文件位置（各平台）**：

| 操作系统 | 配置目录 | 数据目录 |
|---------|---------|---------|
| **Windows** | `%APPDATA%\openhijack\` | `%LOCALAPPDATA%\openhijack\` |
| **Linux/macOS** | `~/.config/openhijack/` | `~/.local/share/openhijack/` |

### 方式一：权限提升模式（推荐）

```bash
./openhijack elevate
```

自动完成权限提升 → 读取配置 → 生成证书 → 安装 CA → 修改 hosts → 启动 HTTPS 代理。

**Windows 特性**：
- 自动弹出 UAC 对话框请求管理员权限
- 无需手动右键"以管理员身份运行"
- 如果 UAC 被拒绝或禁用，会提示降级方案

### 方式二：直接启动

```bash
# 使用默认配置
./openhijack serve

# 自定义配置文件
./openhijack serve --config /path/to/config.toml

# 纯 HTTP 模式（无需管理员权限）
./openhijack serve --http --port 8787

# 不自动管理证书和 hosts（已有证书时）
./openhijack serve --no-manage
```

### 方式三：手动 sudo（Linux/macOS）

```bash
sudo ./openhijack serve
```

如果使用自定义配置路径，也可以显式传 `--config`，或者在 `sudo` 时保留 `OPENHIJACK_CONFIG` 环境变量。

### 查看路径信息

```bash
./openhijack paths
```

输出示例（Windows）：
```
配置目录:     D:\Users\user\AppData\Roaming\openhijack
数据目录:     D:\Users\user\AppData\Local\openhijack
Hosts 文件:   C:\Windows\System32\drivers\etc\hosts
Hosts 备份:   D:\Users\user\AppData\Local\openhijack\hosts.backup
CA 目录:      D:\Users\user\AppData\Local\openhijack\ca
CA 证书:      D:\Users\user\AppData\Local\openhijack\ca\ca.crt
CA 私钥:      D:\Users\user\AppData\Local\openhijack\ca\ca.key
服务器证书:   D:\Users\user\AppData\Local\openhijack\ca\api.openai.com.crt
服务器私钥:   D:\Users\user\AppData\Local\openhijack\ca\api.openai.com.key
```

### 清理安装痕迹

```bash
./openhijack cleanup
# 或
./openhijack uninstall
```

会移除 hosts 条目、系统 CA 信任，以及本地生成的证书和 hosts 备份。

## 命令行选项

```
openhijack init [选项]
  --config string              配置文件路径
  --force                      覆盖已存在的配置文件

openhijack serve [选项]
  --host string                监听地址 (默认: 所有接口)
  --port int                   监听端口 (默认: 443)
  --config string              配置文件路径
  --http                       使用纯 HTTP 模式 (不使用 TLS)
  --no-manage                  不自动管理证书和 hosts
  --debug                      调试模式 (打印请求头/请求体)
  --disable-ssl-strict-mode    禁用上游 TLS 证书校验
  --force-stream               强制使用流模式

openhijack elevate
  权限提升并启动代理（自动 sudo/UAC）

openhijack cleanup
  移除 hosts、系统 CA 和本地证书文件

openhijack paths
  显示数据路径
```

## 配置文件

可通过 `--config` 参数或 `OPENHIJACK_CONFIG` 环境变量指定。

```toml
mapped_model_id = "my-model"
auth_key = "your-auth-key"
current_config_index = 0

[[config_groups]]
name = "provider-a"
provider = "openai_chat_completion"
api_url = "https://provider-a.example.com/api"
model_id = "model-a"
api_key = "your-provider-a-api-key"
middle_route = "/v1"

[[config_groups]]
name = "provider-b"
provider = "openai_chat_completion"
api_url = "https://provider-b.example.com/api"
model_id = "model-b"
api_key = "your-provider-b-api-key"
middle_route = "/v1"
```

通过 `current_config_index` 指定当前使用哪个配置组（从 0 开始）。支持配置多个上游，按需切换。

目前仅实现 `openai_chat_completion`（或别名 `openai`）。`openai_response`、`anthropic`、`gemini` 暂未实现，配置时会直接报错。

| 字段 | 说明 |
|------|------|
| `mapped_model_id` | 对客户端暴露的模型 ID |
| `auth_key` | 客户端鉴权密钥（留空则不鉴权） |
| `current_config_index` | 当前使用的配置组索引 |
| `config_groups[].name` | 配置组名称 |
| `config_groups[].provider` | 上游服务商类型 |
| `config_groups[].api_url` | 上游 API 基础地址 |
| `config_groups[].model_id` | 上游实际模型 ID |
| `config_groups[].api_key` | 上游 API 密钥 |
| `config_groups[].middle_route` | 中间路由路径 |

## Hosts 劫持域名

默认劫持以下域名到本地：

| 域名 | 用途 |
|------|------|
| `api.openai.com` | OpenAI API |
| `openrouter.ai` | OpenRouter API |

如需修改支持的域名列表，请编辑 [internal/hosts/hosts.go](internal/hosts/hosts.go) 中的 `hostsDomains` 变量。

## 文件路径

| 路径 | 说明 |
|------|------|
| `<config_dir>/config.toml` | 配置文件 |
| `<data_dir>/ca/ca.crt` | CA 证书 |
| `<data_dir>/ca/ca.key` | CA 私钥 |
| `<data_dir>/ca/api.openai.com.crt` | 服务器证书 |
| `<data_dir>/ca/api.openai.com.key` | 服务器私钥 |
| `<data_dir>/hosts.backup` | hosts 文件备份 |

**注意**：`<config_dir>` 和 `<data_dir>` 因平台而异（见上方"配置文件位置"表格）。

## Trae 配置

在 Trae 中选择 OpenAI 服务商，填入：

- API Key：配置文件中的 `auth_key`
- Model：配置文件中的 `mapped_model_id`（如 `my-model`）
- Base URL：留空（使用默认 `https://api.openai.com/v1`，会被 hosts 劫持到本地）

Trae 发出的请求会被本地代理拦截并转发到实际上游。

## 项目结构

```
├── cmd/openhijack/main.go       # 程序入口
├── internal/
│   ├── platform/                # ✨ 跨平台抽象层（新增）
│   │   ├── platform.go          # 接口定义和平台检测
│   │   ├── linux.go             # Linux 实现（sudo + syscall.Exec）
│   │   ├── darwin.go            # macOS 实现（同 Linux）
│   │   └── windows.go           # Windows 实现（UAC 自动提权）
│   ├── config/config.go         # TOML 配置加载
│   ├── proxy/
│   │   ├── proxy.go             # 代理服务器、路由、handlers
│   │   ├── transport.go         # 上游转发、SSE 流
│   │   ├── auth.go              # 客户端鉴权
│   │   ├── system_prompt.go     # 系统提示词管理
│   │   └── helpers.go           # 工具函数
│   ├── cert/
│   │   ├── cert.go              # CA + 服务器证书生成
│   │   └── install.go           # 多平台 CA 安装/卸载
│   └── hosts/hosts.go           # 跨平台 hosts 管理
├── Product-Spec.md              # 产品需求文档
├── Product-Spec-CHANGELOG.md    # 变更记录
├── go.mod
└── README.md
```

## 技术架构

### 跨平台设计

采用 Go build tags 条件编译实现平台隔离：

```go
//go:build windows     // 仅在 Windows 编译
//go:build linux       // 仅在 Linux 编译
//go:build darwin      // 仅在 macOS 编译
```

**核心组件**：

| 组件 | 功能 | 平台差异 |
|------|------|---------|
| `platform.IsPrivileged()` | 权限检测 | Unix: `os.Geteuid() == 0`<br>Windows: Token.IsMember(Admin SID) |
| `platform.Elevate()` | 权限提升 | Unix: `exec.Command("sudo", ...)`<br>Windows: `ShellExecute("runas")` UAC |
| `platform.GetHostsPath()` | Hosts 路径 | Unix: `/etc/hosts`<br>Windows: `C:\Windows\System32\drivers\etc\hosts` |
| `platform.GetConfigDir()` | 配置目录 | Unix: `~/.config/openhijack`<br>Windows: `%APPDATA%\openhijack` |
| `platform.GetDataDir()` | 数据目录 | Unix: `~/.local/share/openhijack`<br>Windows: `%LOCALAPPDATA%\openhijack` |

### Windows UAC 提升流程

```
用户执行: openhijack elevate
    ↓
检测 IsPrivileged() == false?
    ↓ 是
ShellExecuteEx("runas", openhijack.exe, elevate ...)
    ↓
Windows 弹出 UAC 对话框
    ↓ 用户点击"是"
新进程以管理员权限启动（继承所有命令行参数和环境变量）
    ↓
当前非特权进程优雅退出 (os.Exit(0))
    ↓
新进程检测到 IsPrivileged() == true
    ↓
直接调用 runServe() 启动服务（无子进程开销）
```

参考实现：[gsudo](https://github.com/gerardog/gsudo) (4000+ stars)

## 依赖

### 编译时依赖

- Go 1.26+
- github.com/BurntSushi/toml
- golang.org/x/sys/windows (仅 Windows 平台)

### 运行时依赖

| 工具 | 适用平台 | 安装方式 |
|------|---------|---------|
| `trust` | Arch Linux | `pacman -S trust` |
| `update-ca-trust` | RHEL/CentOS/Fedora/openSUSE | `yum install ca-certificates` |
| `update-ca-certificates` | Debian/Ubuntu/Alpine | `apt-get install ca-certificates` |
| `security` | macOS | 内置 |
| `certutil` | Windows | 内置 |
| `certctl` | FreeBSD | 内置 |

## 开发

### 运行测试

```bash
# 运行所有测试
go test ./... -v

# 运行特定包测试
go test ./cmd/openhijack/ -v
go test ./internal/hosts/ -v
```

### Lint 检查

```bash
golangci-lint run
```

## License

MIT
