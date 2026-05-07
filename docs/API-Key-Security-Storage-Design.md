# OpenHijack API Key 安全存储方案

**版本**: v1.0  
**日期**: 2026-05-07  
**状态**: 设计文档（待实施）

---

## 📋 目录

1. [背景与问题](#1-背景与问题)
2. [安全威胁模型](#2-安全威胁模型)
3. [解决方案对比](#3-解决方案对比)
4. [推荐方案：混合加密策略](#4-推荐方案混合加密策略)
5. [技术实现细节](#5-技术实现细节)
6. [迁移策略](#6-迁移策略)
7. [风险评估与缓解](#7-风险评估与缓解)

---

## 1. 背景与问题

### 当前状态

OpenHijack 的配置文件使用 **TOML 格式明文存储**所有敏感信息：

```toml
[config_groups]
api_key = "sk-proj-abc123456789xyz..."  # ⚠️ 明文存储！
auth_key = "my-secret-auth-key"          # ⚠️ 明文存储！
```

### 风险场景

| 场景 | 风险等级 | 影响 |
|------|---------|------|
| 配置文件被误提交到 Git | 🔴 高 | 密钥永久泄露 |
| 云备份包含配置文件 | 🔴 高 | 第三方可访问 |
| 多用户环境读取权限 | 🟡 中 | 其他用户可获取密钥 |
| 恶意软件扫描 | 🔴 高 | 自动提取密钥 |
| 物理设备丢失/被盗 | 🔴 高 | 离线攻击可解密 |

### 合规要求

- **OWASP Top 10**: A02:2021 - Cryptographic Failures
- **GDPR**: 敏感数据必须加密存储
- **SOC 2**: 必须有密钥管理策略

---

## 2. 安全威胁模型

### 攻击者类型

1. **本地低特权用户**
   - 能力：读取用户目录下的文件
   - 目标：窃取 API Key
   - 缓解：文件权限 + 加密

2. **恶意软件/勒索软件**
   - 能力：扫描文件系统、内存转储
   - 目标：批量窃取凭证
   - 缓解：运行时加密 + 内存保护

3. **物理访问者**
   - 能力：物理接触设备、启动 Live USB
   - 目标：离线破解
   - 缓解：强加密 + 用户密码绑定

4. **云端备份服务提供商**
   - 能力：访问备份数据
   - 目标：数据挖掘
   - 缓解：端到端加密

### 数据分类

| 数据类型 | 敏感级别 | 存储方式 |
|---------|---------|---------|
| 上游 API Key (OpenAI 等) | 🔴 极高 | 必须加密 |
| Auth Key (代理认证) | 🟡 中高 | 建议加密 |
| 配置路径、URL | 🟢 低 | 可明文 |
| 模型 ID、供应商名称 | 🟢 低 | 可明文 |

---

## 3. 解决方案对比

### 方案 A: 操作系统集成（Keyring）

#### 实现方式
```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│ OpenHijack  │────▶│ Keyring API  │────▶│ OS Key Store │
│ Application │     │ (DBUS/GNOME) │     │ (Encrypted) │
└─────────────┘     └──────────────┘     └─────────────┘
```

#### 优点
- ✅ 利用操作系统原生安全机制
- ✅ 与系统登录密码绑定
- ✅ 自动锁定（屏幕锁定后）
- ✅ 跨应用共享（可选）

#### 缺点
- ❌ Linux 发行版差异大（GNOME/KDE/XFCE）
- ❌ 无 GUI 时不可用（headless server）
- ❌ Windows/Mac 实现不同
- ❌ 依赖外部库（`github.com/zalando/go-keyring`）

#### 适用场景
- 桌面应用（GUI 模式）
- 单用户工作站

---

### 方案 B: 应用级 AES-GCM 加密

#### 实现方式
```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│ Config File │◀────│ Encrypt/     │◀────│ Master Key  │
│ (TOML)      │     │ Decrypt      │     │ (Derived)   │
│             │     │ (AES-256-GCM)│     │             │
│ api_key =   │     └──────────────┘     └─────────────┘
│ "enc:v1:.." │                            ↑
└─────────────┘                      User Password
                                     or Hardware Token
```

#### 优点
- ✅ 跨平台统一实现
- ✅ 不依赖外部服务
- ✅ 可在 headless 环境工作
- ✅ 完全控制加密流程

#### 缺点
- ❌ 需要管理 Master Password
- ❌ 用户需记住额外密码
- ❌ 密码丢失 = 数据丢失

#### 适用场景
- CLI 工具
- Server/容器环境
- 跨平台需求

---

### 方案 C: 混合策略（推荐）✅

结合方案 A 和 B 的优点：

```
                    ┌─────────────────────────────────┐
                    │         OpenHijack Core          │
                    └──────────┬──────────────────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
    ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
    │  Keyring    │  │  File-based │  │  Env Var /  │
    │  (Desktop)  │  │  (Fallback) │  │  CLI Arg    │
    └─────────────┘  └─────────────┘  └─────────────┘
              │                │                │
              └────────────────┼────────────────┘
                               ▼
                    ┌─────────────────────────────────┐
                    │     Unified Crypto Interface     │
                    │     (AES-256-GCM + PBKDF2)      │
                    └─────────────────────────────────┘
```

#### 优点
- ✅ 最佳用户体验（桌面用 Keyring）
- ✅ 最大兼容性（Server 用文件加密）
- ✅ 统一接口（前端代码不变）
- ✅ 渐进式迁移

---

## 4. 推荐方案：混合加密策略

### 架构设计

#### 4.1 存储格式

```toml
# 新的加密存储格式
[config_groups]

# 明文字段（不敏感）
name = "default"
provider = "openai_chat_completion"
api_url = "https://api.openai.com"
model_id = "gpt-4o"
middle_route = "/v1"

# 加密字段（敏感）
api_key = "enc:v1:aes256:gcm:base64iv:base64ciphertext:base64tag"
auth_key = "enc:v1:aes256:gcm:base64iv:base64ciphertext:base64tag"
```

#### 4.2 加密标识符格式

```
enc:{version}:{algorithm}:{mode}:{iv}:{ciphertext}:{tag}
```

示例：
```
enc:v1:aes256:gcm:abc123def456:xyz789...base64...:tag123...
```

#### 4.3 密钥派生流程

```
User Password (or System Keyring)
        │
        ▼
┌───────────────────┐
│ PBKDF2-HMAC-SHA256│
│ Iterations: 600,000│
│ Salt: random 16B   │
│ Output: 32 bytes   │
└─────────┬─────────┘
          │
          ├──────────────┬──────────────┐
          ▼              ▼              ▼
     AES-256-Key    HMAC-Key       (Future Use)
     (Encryption)   (Integrity)
```

---

## 5. 技术实现细节

### 5.1 Go 实现（后端）

```go
// internal/crypto/crypto.go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"golang.org/x/crypto/pbkdf2"
)

const (
	// 版本号
	Version = "v1"
	
	// 加密算法参数
	Algorithm  = "aes256"
	Mode       = "gcm"
	KeySize    = 32 // AES-256
	NonceSize  = 12 // GCM 标准 Nonce 大小
	TagSize    = 16 // GCM Tag 大小
	
	// PBKDF2 参数
	PBKDF2Iterations = 600_000
	SaltSize         = 16
)

// EncryptedValue 加密后的值结构
type EncryptedValue struct {
	Version   string
	Algorithm string
	Mode      string
	IV        string // Base64 编码
	Ciphertext string // Base64 编码
	Tag       string // Base64 编码
}

// Encrypt 加密明文
func Encrypt(plaintext string, masterPassword string) (string, error) {
	// 1. 生成随机 Salt
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt failed: %w", err)
	}
	
	// 2. 派生密钥
	key := pbkdf2.Key(
		[]byte(masterPassword),
		salt,
		PBKDF2Iterations,
		KeySize,
		sha256.New,
	)
	
	// 3. 创建 AES-GCM cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher failed: %w", err)
	}
	
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm failed: %w", err)
	}
	
	// 4. 生成随机 IV/Nonce
	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce failed: %w", err)
	}
	
	// 5. 加密
	ciphertext := aesGCM.Seal(nil, nonce, []byte(plaintext), nil)
	
	// 6. 分离 Ciphertext 和 Tag
	ctLen := len(ciphertext) - TagSize
	ct := ciphertext[:ctLen]
	tag := ciphertext[ctLen:]
	
	// 7. 构建加密值结构
	encrypted := EncryptedValue{
		Version:   Version,
		Algorithm: Algorithm,
		Mode:      Mode,
		IV:        base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ct),
		Tag:       base64.StdEncoding.EncodeToString(tag),
	}
	
	// 8. 序列化为字符串
	result := fmt.Sprintf("enc:%s:%s:%s:%s:%s:%s",
		encrypted.Version,
		encrypted.Algorithm,
		encrypted.Mode,
		encrypted.IV,
		encrypted.Ciphertext,
		encrypted.Tag,
	)
	
	return result, nil
}

// Decrypt 解密密文
func Decrypt(encryptedStr string, masterPassword string) (string, error) {
	// 1. 解析加密字符串
	encrypted, err := parseEncryptedString(encryptedStr)
	if err != nil {
		return "", err
	}
	
	// 2. Base64 解码
	iv, _ := base64.StdEncoding.DecodeString(encrypted.IV)
	ct, _ := base64.StdEncoding.DecodeString(encrypted.Ciphertext)
	tag, _ := base64.StdEncoding.DecodeString(encrypted.Tag)
	
	// 3. 重新组合 Ciphertext + Tag
	ciphertext := append(ct, tag...)
	
	// 4. TODO: 从安全位置获取 Salt（需要存储方案）
	// 这里简化处理，实际应从配置元数据中读取
	salt := []byte{} // 占位符
	
	// 5. 派生密钥
	key := pbkdf2.Key(
		[]byte(masterPassword),
		salt,
		PBKDF2Iterations,
		KeySize,
		sha256.New,
	)
	
	// 6. 创建 cipher 并解密
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher failed: %w", err)
	}
	
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm failed: %w", err)
	}
	
	plaintext, err := aesGCM.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt failed: wrong password or corrupted data")
	}
	
	return string(plaintext), nil
}

// IsEncrypted 检查字符串是否为加密格式
func IsEncrypted(value string) bool {
	return len(value) > 4 && value[:4] == "enc:"
}
```

### 5.2 存储层抽象

```go
// internal/crypto/store.go
package crypto

import (
	"context"
	"os"
	"path/filepath"
)

// SecretStore 密钥存储接口
type SecretStore interface {
	// Get 获取密钥
	Get(ctx context.Context, key string) (string, error)
	
	// Set 设置密钥
	Set(ctx context.Context, key, value string) error
	
	// Delete 删除密钥
	Delete(ctx context.Context, key string) error
	
	// Exists 检查密钥是否存在
	Exists(ctx context.Context, key string) (bool, error)
}

// KeyringStore 操作系统 Keyring 存储
type KeyringStore struct {
	serviceName string
	userName    string
}

// FileStore 文件级加密存储（备用方案）
type FileStore struct {
	basePath string
	masterKey []byte
}

// EnvVarStore 环境变量存储（开发/CI 用）
type EnvVarStore struct {
	prefix string
}

// NewSecretStore 根据环境选择最佳存储后端
func NewSecretStore() (SecretStore, error) {
	// 优先级：
	// 1. 环境变量 OPENHIJACK_KEYSTORE=env → EnvVarStore
	// 2. 检测 Desktop 环境 → KeyringStore
	// 3. 默认 → FileStore
	
	if os.Getenv("OPENHIJACK_KEYSTORE") == "env" {
		return &EnvVarStore{prefix: "OPENHIJACK_SECRET_"}, nil
	}
	
	if isDesktopEnvironment() {
		store, err := NewKeyringStore()
		if err == nil {
			return store, nil
		}
	}
	
	// Fallback to file-based store
	return NewFileStore()
}
```

### 5.3 TOML 序列化集成

```go
// internal/config/crypto_config.go
package config

import (
	"openhijack/internal/crypto"
)

// SecureConfigGroup 安全的配置组（支持加密字段）
type SecureConfigGroup struct {
	Name        string `toml:"name"`
	Provider    string `toml:"provider"`
	APIUrl      string `toml:"api_url"`
	ModelID     string `toml:"model_id"`
	MiddleRoute string `toml:"middle_route"`
	
	// 加密字段
	APIKeyRaw  string `toml:"api_key"`  // 可能是明文或加密格式
	AuthKeyRaw string `toml:"auth_key"` // 可能是明文或加密格式
}

// GetAPIKey 获取解密后的 API Key
func (g *SecureConfigGroup) GetAPIKey(masterPassword string) (string, error) {
	if crypto.IsEncrypted(g.APIKeyRaw) {
		return crypto.Decrypt(g.APIKeyRaw, masterPassword)
	}
	return g.APIKeyRaw, nil // 兼容旧格式
}

// SetAPIKey 设置 API Key（自动加密）
func (g *SecureConfigGroup) SetAPIKey(apiKey, masterPassword string) error {
	encrypted, err := crypto.Encrypt(apiKey, masterPassword)
	if err != nil {
		return err
	}
	g.APIKeyRaw = encrypted
	return nil
}

// NeedsMigration 检查是否需要迁移到加密格式
func (g *SecureConfigGroup) NeedsMigration() bool {
	return !crypto.IsEncrypted(g.APIKeyRaw) && g.APIKeyRaw != ""
}
```

---

## 6. 迁移策略

### Phase 1: 向后兼容（Week 1）

**目标**: 不破坏现有配置文件

```go
// 读取时兼容两种格式
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{}
	
	// 正常加载 TOML
	if _, err := toml.Decode(content, cfg); err != nil {
		return nil, err
	}
	
	// 检测是否有未加密的敏感字段
	for i, group := range cfg.ConfigGroups {
		if group.NeedsMigration() {
			log.Printf("WARNING: config[%d] contains unencrypted secrets", i)
			// 可选：提示用户升级
		}
	}
	
	return cfg, nil
}
```

### Phase 2: 自动加密（Week 2-3）

**目标**: 保存时自动加密新输入的密钥

```go
func SaveConfig(cfg *Config, path string, masterPassword string) error {
	for i := range cfg.ConfigGroups {
		group := &cfg.ConfigGroups[i]
		
		// 如果是新输入的明文，自动加密
		if group.NeedsMigration() && masterPassword != "" {
			if err := group.SetAPIKey(group.APIKeyRaw, masterPassword); err != nil {
				return err
			}
			if err := group.SetAuthKey(group.AuthKeyRaw, masterPassword); err != nil {
				return err
			}
			
			log.Printf("INFO: config[%d] secrets encrypted", i)
		}
	}
	
	// 写入文件
	return writeTOML(cfg, path)
}
```

### Phase 3: 强制加密（Week 4+）

**目标**: 拒绝保存明文密钥

```go
func ValidateConfig(cfg *Config) error {
	for i, group := range cfg.ConfigGroups {
		if group.NeedsMigration() {
			return fmt.Errorf(
				"config[%d]: security policy requires encryption, "+
				"please set OPENHIJACK_MASTER_PASSWORD or use keyring",
				i,
			)
		}
	}
	return nil
}
```

---

## 7. 风险评估与缓解

### 已识别风险

| # | 风险 | 概率 | 影响 | 缓解措施 |
|---|------|------|------|---------|
| R1 | Master Password 弱 | 高 | 高 | 强制最小长度 + 复杂度检查 |
| R2 | PBKDF2 迭代次数不足 | 低 | 高 | 使用 OWASP 推荐值 (600K+) |
| R3 | Salt 重用 | 极低 | 高 | 每次加密生成随机 Salt |
| R4 | 内存中的明文残留 | 中 | 中 | 使用完毕后清零 (`crypto.Zeroable`) |
| R5 | 侧信道攻击（时序） | 低 | 高 | 使用 Go 标准库的常量时间比较 |
| R6 | 密钥备份丢失 | 中 | 高 | 提供导出/导入功能 + 恢复指南 |

### 安全 Checklist

- [ ] 使用 AES-256-GCM（认证加密）
- [ ] PBKDF2 迭代 ≥ 600,000 次
- [ ] 每次加密使用随机 IV/Nonce
- [ ] Salt 随机生成且唯一
- [ ] 密码强度验证（≥12 字符，含大小写+数字+特殊字符）
- [ ] 错误消息不泄露信息（通用 "decryption failed"）
- [ ] 日志中不记录明文或密文
- [ ] 内存使用后及时清理

---

## 📚 参考资料

1. [OWASP Cryptographic Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cryptographic_Storage_Cheat_Sheet.html)
2. [NIST SP 800-63B](https://pages.nist.gov/800-63-3/sp80063b.html) - Digital Identity Guidelines
3. [Go Crypto Best Practices](https://go.dev/blog/tls-cipher-suites)
4. [AES-GCM-SIV Specification](https://datatracker.ietf.org/doc/html/rfc8452)

---

**文档维护者**: AI Code Review Assistant  
**下次更新**: 实施完成后进行安全审计
