# 修复 GUI 版本证书安装失败问题 Spec

## Why
GUI 版本在启动代理服务时证书安装失败，而命令行版本正常工作。通过分析代码发现两个版本在处理数据目录路径、权限提升后的环境变量传递等方面存在差异，导致 GUI 版本无法正确安装 CA 证书到系统信任库。

## What Changes
- 分析并修复 GUI 版本与命令行版本在证书管理流程中的差异
- 确保 GUI 版本在 sudo 模式下能正确解析用户主目录和数据目录
- 统一两个版本的证书生成、安装逻辑
- 优化错误提示信息，帮助用户诊断证书问题

## Impact
- Affected specs: 证书管理系统
- Affected code:
  - `gui/app.go` - StartProxy 方法、resolveHomeDir 方法、getDataDir 方法
  - `internal/cert/install.go` - InstallCACert 方法
  - 可能涉及 `internal/platform/` 平台相关代码

## AD Requirements
### Requirement: 统一数据目录解析逻辑
GUI 版本必须与命令行版本使用相同的数据目录解析逻辑，特别是在 sudo 模式下。

#### Scenario: sudo 模式下启动服务
- **WHEN** 用户通过 `sudo` 启动 GUI 应用并启动代理服务
- **THEN** 系统应正确解析原始用户的主目录（而非 /root）
- **THEN** CA 证书应成功安装到系统信任库
- **THEN** 服务器证书应在正确的数据目录下生成

### Requirement: 改进证书安装错误处理
GUI 版本应提供更详细的证书安装失败信息，帮助用户快速定位问题。

#### Scenario: 证书安装权限不足
- **WHEN** 用户以普通用户身份运行 GUI 并尝试启动代理
- **THEN** 如果证书安装因权限不足失败，应明确提示需要 sudo 权限
- **THEN** 提供具体的解决方案建议

### Requirement: 保持向后兼容
修复不应破坏现有命令行版本的功能。

#### Scenario: 命令行版本正常运行
- **WHEN** 用户使用命令行版本启动服务
- **THEN** 所有现有功能保持不变
- **THEN** 证书管理流程不受影响

## MODIFIED Requirements
### Requirement: GUI 版本 StartProxy 方法
修改 `gui/app.go` 中的 `StartProxy` 方法：
- 使用与命令行版本一致的数据目录解析方式
- 增强证书安装失败的错误信息和恢复建议
- 确保在 sudo 模式下正确传递环境变量

### Requirement: GUI 版本 resolveHomeDir 方法
验证或修改 `gui/app.go` 中的 `resolveHomeDir` 方法：
- 确保在所有场景下（普通用户、sudo、root）都能正确返回用户主目录
- 与 `platform.ResolveHomeDir` 保持一致的逻辑
