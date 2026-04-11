# OpenHijack

本地 HTTPS 代理服务器，用于让仅支持固定服务商（如 OpenAI）的客户端（如 Trae IDE）通过本地代理实际使用自选上游服务。

## 工作原理

```
客户端 (Trae) → https://api.openai.com:443
                        ↓ (hosts 劫持)
              127.0.0.1:443 (本地 HTTPS 代理)
                        ↓ (转发到真实上游)
              https://your-upstream-provider/v1/...
```

1. 生成自签名 CA 证书（CN=OpenHijack_CA），签发 `api.openai.com` 服务器证书
2. 安装 CA 到系统信任库
3. 修改 `/etc/hosts`，将 `api.openai.com` 指向 `127.0.0.1`
4. 在本地 443 端口启动 HTTPS 服务器
5. 拦截所有发往 `api.openai.com` 的请求，转发到配置的上游地址
   - `GET /v1/models` — 返回映射后的模型信息
   - `POST /v1/chat/completions` — 转发到上游（支持 SSE 流式/非流式）
   - 其他路径 — 直接透传到上游
6. 停止时自动清理 hosts 条目

## 功能

- OpenAI 兼容 API 代理（模型 ID 映射）
- SSE 流式 / 非流式响应透传
- 客户端鉴权
- 系统提示词捕获与覆盖
- 自动 CA 证书生成与系统安装
- 自动 hosts 文件管理
- 权限提升（sudo）
- 多平台 CA 安装（Arch Linux / Debian / Ubuntu / RHEL / Fedora / macOS / Windows）

## 构建

```bash
go build -o openhijack ./cmd/openhijack/
```

## 使用

### 方式一：权限提升模式（推荐）

```bash
./openhijack elevate
```

自动完成 sudo → 生成证书 → 安装 CA → 修改 hosts → 启动 HTTPS 代理。

### 方式二：手动 sudo

```bash
sudo ./openhijack serve
```

如果使用自定义配置路径，也可以显式传 `--config`，或者在 `sudo` 时保留 `OPENHIJACK_CONFIG`。

### 方式三：纯 HTTP 测试

```bash
./openhijack serve --http --port 8787
```

不需要 root，不管理证书和 hosts，适合调试。

### 查看路径

```bash
./openhijack paths
```

## 命令行选项

```
openhijack serve [选项]
  --host string                监听地址 (默认: 所有接口)
  --port int                   监听端口 (默认: 443)
  --config string              配置文件路径
  --http                       使用纯 HTTP 模式 (不使用 TLS)
  --no-manage                  不自动管理证书和 hosts
  --debug                      调试模式 (打印请求头/请求体)
  --disable-ssl-strict-mode    禁用上游 TLS 证书校验
  --force-stream               强制使用流模式
```

## 配置文件

默认路径：`~/.config/openhijack/config.toml`

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

## 文件路径

| 路径 | 说明 |
|------|------|
| `~/.config/openhijack/config.toml` | 配置文件 |
| `~/.local/share/openhijack/ca/ca.crt` | CA 证书 |
| `~/.local/share/openhijack/ca/ca.key` | CA 私钥 |
| `~/.local/share/openhijack/ca/api.openai.com.crt` | 服务器证书 |
| `~/.local/share/openhijack/ca/api.openai.com.key` | 服务器私钥 |
| `~/.local/share/openhijack/hosts.backup` | hosts 文件备份 |

## Trae 配置

在 Trae 中选择 OpenAI 服务商，填入：

- API Key：配置文件中的 `auth_key`
- Model：配置文件中的 `mapped_model_id`（如 `my-model`）

Trae 发出的请求会被本地代理拦截并转发到实际上游。

## 项目结构

```
├── cmd/openhijack/main.go       # 程序入口
├── internal/
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
│   └── hosts/hosts.go           # /etc/hosts 管理
├── go.mod
└── README.md
```

## 依赖

- Go 1.26+
- github.com/BurntSushi/toml
- 运行时：`trust` (Arch Linux) / `update-ca-certificates` (Debian) / `security` (macOS) / `certutil` (Windows)
