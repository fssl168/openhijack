package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	rt "runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"openhijack/internal/cert"
	"openhijack/internal/config"
	"openhijack/internal/hosts"
	"openhijack/internal/platform"
	"openhijack/internal/proxy"

	"github.com/BurntSushi/toml"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var supportedProviders = map[string]string{
	"openai_chat_completion": "OpenAI 兼容 API",
	"openai_response":        "OpenAI Response API",
	"anthropic":              "Anthropic Claude",
	"gemini":                 "Google Gemini",
	"openrouter":             "OpenRouter 聚合 API",
}

var providerDefaults = map[string]map[string]string{
	"openai_chat_completion": {
		"api_url":      "https://api.openai.com",
		"middle_route": "/v1",
	},
	"openrouter": {
		"api_url":      "https://openrouter.ai/api",
		"middle_route": "/v1",
	},
	"anthropic": {
		"api_url":      "https://api.anthropic.com",
		"middle_route": "/v1",
	},
	"gemini": {
		"api_url":      "https://generativelanguage.googleapis.com",
		"middle_route": "/v1beta",
	},
	"openai_response": {
		"api_url":      "https://api.openai.com",
		"middle_route": "/v1",
	},
}

type App struct {
	ctx        context.Context
	server     *proxy.ProxyServer
	mu         sync.RWMutex
	running    bool
	startTime  time.Time
	lastConfig string
	lastPort   int
	dataDir    string
	logBuffer  *LogBuffer
	logChan    chan string
	cleanupFn  func()
}

type StatusInfo struct {
	Running  bool   `json:"running"`
	Port     int    `json:"port"`
	Host     string `json:"host"`
	Config   string `json:"config"`
	Uptime   string `json:"uptime"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

type ConfigInfo struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Active   bool   `json:"active"`
}

type ConfigGroupData struct {
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	APIURL      string `json:"api_url"`
	ModelID     string `json:"model_id"`
	APIKey      string `json:"api_key"`
	MiddleRoute string `json:"middle_route"`
}

type ConfigData struct {
	Path          string            `json:"path"`
	MappedModelID string            `json:"mapped_model_id"`
	AuthKey       string            `json:"auth_key"`
	ConfigGroups  []ConfigGroupData `json:"config_groups"`
}

type TestResult struct {
	Success bool   `json:"success"`
	Latency string `json:"latency"`
	Message string `json:"message"`
}

type PlatformInfo struct {
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	Privileged bool   `json:"privileged"`
	HasSudo    bool   `json:"has_sudo"`
	CapSupport bool   `json:"cap_support"`
}

type SystemInfo struct {
	Platform   PlatformInfo `json:"platform"`
	GoVersion  string       `json:"go_version"`
	AppVersion string       `json:"app_version"`
}

type LogBuffer struct {
	mu     sync.RWMutex
	buffer []string
	max    int
}

func NewLogBuffer(max int) *LogBuffer {
	return &LogBuffer{
		buffer: make([]string, 0, max),
		max:    max,
	}
}

func (lb *LogBuffer) Append(line string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.buffer = append(lb.buffer, line)
	if len(lb.buffer) > lb.max {
		lb.buffer = lb.buffer[len(lb.buffer)-lb.max:]
	}
}

func (lb *LogBuffer) Get(limit int) []string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	n := limit
	if n > len(lb.buffer) {
		n = len(lb.buffer)
	}
	return append([]string{}, lb.buffer[len(lb.buffer)-n:]...)
}

func NewApp() *App {
	return &App{
		logBuffer: NewLogBuffer(1000),
		logChan:   make(chan string, 100),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	a.logProxy("OpenHijack GUI 启动中...")
	a.logProxy("操作系统: %s/%s", rt.GOOS, rt.GOARCH)
	a.logProxy("用户: uid=%d euid=%d", os.Getuid(), os.Geteuid())

	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		a.logProxy("SUDO_USER: %s (sudo 模式)", sudoUser)
	}

	if display := os.Getenv("DISPLAY"); display != "" {
		a.logProxy("DISPLAY: %s", display)
	} else {
		a.logProxy("警告: DISPLAY 环境变量未设置")
	}

	if xauth := os.Getenv("XAUTHORITY"); xauth != "" {
		a.logProxy("XAUTHORITY: %s", xauth)
	}

	go a.processLogStream()
	go a.detectAutoStart()
}

func (a *App) RunElevated() error {
	if !platform.IsPrivileged() {
		args := []string{"elevate"}
		args = append(args, os.Args[2:]...)

		env := os.Environ()
		configDir, err := platform.GetConfigDir()
		if err == nil {
			defaultConfigPath := filepath.Join(configDir, "config.toml")
			env = append(env, fmt.Sprintf("OPENHIJACK_CONFIG=%s", defaultConfigPath))
		}

		return platform.Elevate(args, env)
	}

	configSrc := os.Getenv("OPENHIJACK_CONFIG")
	if configSrc == "" {
		configDir, err := platform.GetConfigDir()
		if err != nil {
			return fmt.Errorf("无法获取配置目录: %w", err)
		}
		configSrc = filepath.Join(configDir, "config.toml")
	}

	if _, err := os.Stat(configSrc); os.IsNotExist(err) {
		return fmt.Errorf("配置文件不存在: %s\n请先运行 GUI 初始化配置或手动创建配置文件", configSrc)
	}

	adminConfigDir, err := platform.GetConfigDir()
	if err != nil {
		return fmt.Errorf("无法获取管理员配置目录: %w", err)
	}

	if err := platform.EnsureDir(adminConfigDir, 0700); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	rootConfigFile := filepath.Join(adminConfigDir, "config.toml")
	srcData, err := os.ReadFile(configSrc)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	if err := os.WriteFile(rootConfigFile, srcData, 0600); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	fmt.Printf("✓ Elevate 模式已激活\n")
	fmt.Printf("  配置文件: %s\n", rootConfigFile)
	fmt.Printf("  用户权限: root (euid=0)\n")
	fmt.Printf("  SUDO_USER: %s\n", os.Getenv("SUDO_USER"))

	dataDir, err := platform.GetDataDir()
	if err != nil {
		return fmt.Errorf("无法获取数据目录: %w", err)
	}
	a.dataDir = dataDir
	a.lastConfig = rootConfigFile

	fmt.Printf("  数据目录: %s\n", dataDir)

	certMgr := cert.NewCertManager(dataDir)

	if !certMgr.HasCA() {
		fmt.Println("  正在生成 CA 证书...")
		if err := certMgr.GenerateCA(func(s string, i ...interface{}) { fmt.Printf("    "+s+"\n", i...) }); err != nil {
			return fmt.Errorf("生成 CA 证书失败: %w", err)
		}
		fmt.Println("    ✓ CA 证书生成成功")
	}

	if !certMgr.HasServerCert() {
		fmt.Println("  正在生成服务器证书...")
		if err := certMgr.GenerateServerCert(func(s string, i ...interface{}) { fmt.Printf("    "+s+"\n", i...) }); err != nil {
			return fmt.Errorf("生成服务器证书失败: %w", err)
		}
		if err := certMgr.GenerateOpenRouterCert(func(s string, i ...interface{}) { fmt.Printf("    "+s+"\n", i...) }); err != nil {
			return fmt.Errorf("生成 OpenRouter 证书失败: %w", err)
		}
		fmt.Println("    ✓ 服务器证书生成成功")
	}

	fmt.Println("  正在安装 CA 证书到系统...")
	if err := cert.InstallCACert(certMgr.CACertFile(), func(s string, i ...interface{}) { fmt.Printf("    "+s+"\n", i...) }); err != nil {
		fmt.Printf("    ⚠ CA 证书安装失败 (非致命): %v\n", err)
		fmt.Println("    提示: 不安装 CA 证书也能正常使用，只是浏览器会提示不安全")
	} else {
		fmt.Println("    ✓ CA 证书安装成功")
	}

	hostsMgr := hosts.NewHostsManager(dataDir)
	fmt.Println("  正在修改 hosts 文件...")
	if err := hostsMgr.AddEntry(func(s string, i ...interface{}) { fmt.Printf("    "+s+"\n", i...) }); err != nil {
		fmt.Printf("    ⚠ hosts 修改失败 (可忽略): %v\n", err)
	} else {
		fmt.Println("    ✓ hosts 文件已更新")
	}

	tlsCertFile, tlsKeyFile := certMgr.TLSCert()

	dataDirForCleanup := dataDir
	a.cleanupFn = func() {
		fmt.Println("\n清理: 移除系统资源...")
		cleanupHostsMgr := hosts.NewHostsManager(dataDirForCleanup)
		if err := cleanupHostsMgr.RemoveEntry(func(s string, i ...interface{}) { fmt.Printf("  "+s+"\n", i...) }); err != nil {
			fmt.Printf("  清理 hosts 失败: %v\n", err)
		}
		fmt.Println("  移除系统 CA 信任...")
		cert.RemoveCACert(func(s string, i ...interface{}) { fmt.Printf("  "+s+"\n", i...) })
	}

	port := 443
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == "--port" && i+1 < len(os.Args) {
			fmt.Sscanf(os.Args[i+1], "%d", &port)
		}
	}

	opts := proxy.ServeOptions{
		ConfigPath:    rootConfigFile,
		Host:          "0.0.0.0",
		Port:          port,
		UseTLS:        true,
		DebugMode:     false,
		ForceStream:   false,
		TLSCertFile:   tlsCertFile,
		TLSKeyFile:    tlsKeyFile,
		ExtraTLSCerts: certMgr.ExtraTLSCerts(),
		CleanupFn:     a.cleanupFn,
	}

	server, err := proxy.NewProxyServer(opts)
	if err != nil {
		return fmt.Errorf("创建代理服务器失败: %w", err)
	}

	fmt.Printf("\n🚀 启动代理服务 (端口: %d)...\n", port)
	if err := server.Start(); err != nil {
		return fmt.Errorf("启动代理服务器失败: %w", err)
	}

	a.server = server
	a.running = true
	a.startTime = time.Now()
	a.lastPort = port

	fmt.Println("✓ 代理服务已启动并运行")
	fmt.Println("\n按 Ctrl+C 停止服务...")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n正在停止服务...")
	a.StopProxy()
	fmt.Println("✓ 服务已停止")

	return nil
}

func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.server != nil {
		a.StopProxy()
	}
	close(a.logChan)
}

func (a *App) detectAutoStart() {
	time.Sleep(500 * time.Millisecond)
	a.GetStatus()
}

func (a *App) processLogStream() {
	for log := range a.logChan {
		a.logBuffer.Append(log)
	}
}

func (a *App) logProxy(msg string, args ...interface{}) {
	if len(args) > 0 {
		a.logChan <- fmt.Sprintf(msg, args...)
	} else {
		a.logChan <- msg
	}
	select {
	case <-a.ctx.Done():
		return
	default:
	}
}

func (a *App) GetStatus() StatusInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	status := StatusInfo{
		Running: a.running,
		Port:    a.lastPort,
		Host:    "localhost",
		Config:  a.lastConfig,
	}

	if a.lastConfig != "" {
		if cfg, err := config.Load(a.lastConfig); err == nil {
			group := cfg.CurrentGroup()
			status.Model = group.ModelID
			status.Provider = group.Provider
		}
	}

	if a.running {
		status.Uptime = time.Since(a.startTime).Round(time.Second).String()
	}

	return status
}

func (a *App) GetConfigs() []ConfigInfo {
	configs := a.discoverConfigs()
	a.mu.RLock()
	activeConfig := a.lastConfig
	a.mu.RUnlock()

	for i := range configs {
		configs[i].Active = configs[i].Path == activeConfig
	}

	return configs
}

func (a *App) discoverConfigs() []ConfigInfo {
	var configs []ConfigInfo

	configDirs := a.getConfigSearchDirs()
	for _, dir := range configDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			if !strings.HasSuffix(entry.Name(), ".toml") {
				continue
			}

			path := filepath.Join(dir, entry.Name())
			cfg, err := config.Load(path)
			if err != nil {
				continue
			}

			group := cfg.CurrentGroup()
			info := ConfigInfo{
				Path:     path,
				Name:     cfg.MappedModelID,
				Provider: group.Provider,
				Model:    group.ModelID,
			}

			configs = append(configs, info)
		}
	}

	return configs
}

func (a *App) getConfigSearchDirs() []string {
	var dirs []string

	home := a.resolveHomeDir()
	if home != "" {
		dirs = append(dirs,
			filepath.Join(home, ".config", "openhijack"),
			filepath.Join(home, ".openhijack"),
		)
	}

	if platform.IsPrivileged() {
		dirs = append(dirs, "/etc/openhijack")
	}

	return dirs
}

func (a *App) CreateConfig(data ConfigData) string {
	if data.Path == "" {
		return "配置文件路径不能为空"
	}

	data.Path = a.resolveConfigPath(data.Path)

	dir := filepath.Dir(data.Path)
	if err := mkdirAll(dir); err != nil {
		return fmt.Sprintf("创建配置目录失败: %v", err)
	}

	content := a.buildConfigContent(data)
	if err := os.WriteFile(data.Path, []byte(content), 0600); err != nil {
		return fmt.Sprintf("写入配置文件失败: %v", err)
	}

	return ""
}

func (a *App) UpdateConfig(data ConfigData) string {
	if data.Path == "" {
		return "配置文件路径不能为空"
	}

	data.Path = a.resolveConfigPath(data.Path)

	if _, err := os.Stat(data.Path); os.IsNotExist(err) {
		return "配置文件不存在"
	}

	content := a.buildConfigContent(data)
	if err := os.WriteFile(data.Path, []byte(content), 0600); err != nil {
		return fmt.Sprintf("更新配置文件失败: %v", err)
	}

	return ""
}

func (a *App) DeleteConfig(path string) string {
	if path == "" {
		return "配置文件路径不能为空"
	}

	a.mu.RLock()
	isActive := path == a.lastConfig
	a.mu.RUnlock()

	if isActive && a.running {
		return "无法删除当前活跃配置（请先停止服务）"
	}

	if err := os.Remove(path); err != nil {
		return fmt.Sprintf("删除配置文件失败: %v", err)
	}

	return ""
}

func (a *App) SelectConfig(configPath string) string {
	if configPath == "" {
		return "配置路径不能为空"
	}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Sprintf("配置文件不存在: %s", configPath)
	}
	a.mu.Lock()
	a.lastConfig = configPath
	a.mu.Unlock()
	return ""
}

func (a *App) StartProxy(configPath string, port int) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.server != nil {
		a.logProxy("检测到残留服务实例，正在清理...")
		a.cleanupAndStop()
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Sprintf("配置文件不存在: %s", configPath)
	}

	if port < 1024 && os.Geteuid() != 0 {
		return fmt.Sprintf("端口 %d 需要 root 权限，请使用 ≥1024 的端口 (如 8443) 或以 sudo 启动应用", port)
	}

	effectiveConfigPath := a.prepareConfigForPrivilegedMode(configPath)

	dataDir := a.getDataDir()
	certMgr := cert.NewCertManager(dataDir)
	hostsMgr := hosts.NewHostsManager(dataDir)

	var tlsCertFile, tlsKeyFile string

	if !certMgr.HasCA() {
		a.logProxy("CA 证书不存在，开始生成...")
		if err := certMgr.GenerateCA(a.logProxy); err != nil {
			return fmt.Sprintf("生成 CA 证书失败: %v", err)
		}
	}

	if !certMgr.HasServerCert() {
		a.logProxy("服务器证书不存在，开始生成...")
		if err := certMgr.GenerateServerCert(a.logProxy); err != nil {
			return fmt.Sprintf("生成服务器证书失败: %v", err)
		}
		if err := certMgr.GenerateOpenRouterCert(a.logProxy); err != nil {
			return fmt.Sprintf("生成 OpenRouter 证书失败: %v", err)
		}
	}

	caCertPath := certMgr.CACertFile()
	a.logProxy("准备安装 CA 证书到系统...")
	a.logProxy("CA 证书路径: %s", caCertPath)
	a.logProxy("用户权限信息: uid=%d euid=%d", os.Getuid(), os.Geteuid())
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		a.logProxy("SUDO_USER: %s (sudo 模式)", sudoUser)
	}

	if err := cert.InstallCACert(caCertPath, a.logProxy); err != nil {
		errStr := err.Error()
		a.logProxy("⚠️ 安装 CA 证书失败: %v", err)

		var solutions []string

		if strings.Contains(errStr, "permission denied") || strings.Contains(errStr, "权限") || strings.Contains(errStr, "denied") || strings.Contains(errStr, "Permission") {
			solutions = append(solutions,
				fmt.Sprintf("1. 权限不足：请使用 sudo 启动应用\n   sudo ./scripts/start-gui.sh\n   或运行: sudo %s elevate", os.Args[0]))
		}

		if strings.Contains(errStr, "not found") || strings.Contains(errStr, "command not found") || strings.Contains(errStr, "No such file") || strings.Contains(errStr, "executable file not found") {
			solutions = append(solutions,
				"2. 系统工具缺失：请安装 ca-certificates 包\n"+
					"   Debian/Ubuntu: sudo apt-get install ca-certificates\n"+
					"   RHEL/CentOS: sudo yum install ca-certificates\n"+
					"   Arch Linux: sudo pacman -S ca-certificates")
		}

		solutions = append(solutions,
			fmt.Sprintf("3. 手动安装（可选）：\n"+
				"   sudo cp %s /usr/local/share/ca-certificates/openhijack-ca.crt\n"+
				"   sudo update-ca-certificates", caCertPath))

		errorMsg := fmt.Sprintf("安装 CA 证书到系统失败: %v\n\n", err)
		errorMsg += "可能的原因和解决方案：\n"
		for _, sol := range solutions {
			errorMsg += fmt.Sprintf("\n%s\n", sol)
		}
		errorMsg += "\n注意：即使不安装 CA 证书，代理服务也能正常运行，\n      只是浏览器会提示证书不安全（需要手动信任）。"

		return errorMsg
	}
	a.logProxy("✓ CA 证书已成功安装到系统")

	if err := hostsMgr.AddEntry(a.logProxy); err != nil {
		a.logProxy(fmt.Sprintf("修改 hosts 文件失败 (可忽略): %v", err))
	}

	tlsCertFile, tlsKeyFile = certMgr.TLSCert()

	dataDirForCleanup := dataDir
	a.cleanupFn = func() {
		a.logProxy("清理: 移除 hosts 条目...")
		cleanupHostsMgr := hosts.NewHostsManager(dataDirForCleanup)
		if err := cleanupHostsMgr.RemoveEntry(a.logProxy); err != nil {
			a.logProxy(fmt.Sprintf("清理 hosts 失败: %v", err))
		}

		a.logProxy("清理: 移除系统 CA 信任...")
		cert.RemoveCACert(a.logProxy)
	}

	opts := proxy.ServeOptions{
		ConfigPath:    effectiveConfigPath,
		Host:          "0.0.0.0",
		Port:          port,
		UseTLS:        true,
		DebugMode:     false,
		ForceStream:   false,
		TLSCertFile:   tlsCertFile,
		TLSKeyFile:    tlsKeyFile,
		ExtraTLSCerts: certMgr.ExtraTLSCerts(),
		CleanupFn:     a.cleanupFn,
	}

	server, err := proxy.NewProxyServer(opts)
	if err != nil {
		return fmt.Sprintf("创建代理服务器失败: %v", err)
	}

	go func() {
		if err := server.Start(); err != nil {
			errMsg := fmt.Sprintf("代理服务器错误: %v", err)
			if strings.Contains(err.Error(), "permission denied") || strings.Contains(err.Error(), "bind:") {
				errMsg = fmt.Sprintf("端口 %d 绑定失败 (权限不足): %v\n建议: 使用端口 8443 或以 sudo 运行应用", port, err)
			}
			a.logProxy(errMsg)
			a.mu.Lock()
			a.running = false
			a.mu.Unlock()
		}
	}()

	time.Sleep(500 * time.Millisecond)

	a.server = server
	a.running = true
	a.startTime = time.Now()
	a.lastConfig = configPath
	a.lastPort = port

	a.logProxy("代理服务已启动")

	return ""
}

func (a *App) StopProxy() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.cleanupAndStop()

	wasRunning := a.running
	a.running = false

	if wasRunning {
		a.logProxy("代理服务已停止")
	}

	return ""
}

func (a *App) cleanupAndStop() {
	if a.server != nil {
		a.logProxy("正在停止代理服务...")
		a.server.Stop()
		a.server = nil
	}

	if a.cleanupFn != nil {
		a.cleanupFn()
		a.cleanupFn = nil
	}

	a.running = false
}

func (a *App) GetLogs(limit int) []string {
	return a.logBuffer.Get(limit)
}

func (a *App) TestConnection(configPath string) TestResult {
	start := time.Now()

	cfg, err := config.Load(configPath)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("加载配置失败: %v", err),
		}
	}

	group := cfg.CurrentGroup()
	baseURL := group.FullAPIURL("models")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("创建请求失败: %v", err),
		}
	}

	if group.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+group.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return TestResult{
			Success: false,
			Latency: time.Since(start).Round(time.Millisecond).String(),
			Message: fmt.Sprintf("连接失败: %v", err),
		}
	}
	defer resp.Body.Close()

	latency := time.Since(start).Round(time.Millisecond)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return TestResult{
			Success: true,
			Latency: latency.String(),
			Message: fmt.Sprintf("连接成功! 响应状态: %d", resp.StatusCode),
		}
	}

	return TestResult{
		Success: false,
		Latency: latency.String(),
		Message: fmt.Sprintf("连接失败: 响应状态 %d", resp.StatusCode),
	}
}

func (a *App) GetSystemInfo() SystemInfo {
	return SystemInfo{
		Platform: PlatformInfo{
			OS:         rt.GOOS,
			Arch:       rt.GOARCH,
			Privileged: platform.IsPrivileged(),
			HasSudo:    os.Getenv("SUDO_USER") != "",
			CapSupport: rt.GOOS == "linux",
		},
		GoVersion:  rt.Version(),
		AppVersion: "1.0.0",
	}
}

type RuntimeEnv struct {
	UID        int      `json:"uid"`
	EUID       int      `json:"euid"`
	SUDOUser   string   `json:"sudo_user"`
	DISPLAY    string   `json:"display"`
	XAUTHORITY string   `json:"xauthority"`
	HOME       string   `json:"home"`
	Warnings   []string `json:"warnings"`
}

func (a *App) GetRuntimeEnv() RuntimeEnv {
	env := RuntimeEnv{
		UID:      os.Getuid(),
		EUID:     os.Geteuid(),
		SUDOUser: os.Getenv("SUDO_USER"),
		DISPLAY:  os.Getenv("DISPLAY"),
		HOME:     a.resolveHomeDir(),
	}

	if xauth := os.Getenv("XAUTHORITY"); xauth != "" {
		env.XAUTHORITY = xauth
	}

	if env.DISPLAY == "" {
		env.Warnings = append(env.Warnings, "DISPLAY 环境变量未设置，GUI 可能无法正常显示")
	}

	if env.EUID == 0 && env.SUDOUser == "" {
		env.Warnings = append(env.Warnings, "以 root 身份运行但未通过 sudo，可能缺少用户环境变量")
	}

	if rt.GOOS == "linux" && env.EUID == 0 && env.DISPLAY != "" {
		env.Warnings = append(env.Warnings, "提示: 以 root 运行 GUI 时确保已正确设置 DISPLAY 和 XAUTHORITY")
	}

	return env
}

func (a *App) InstallCACert() string {
	dataDir := a.getDataDir()
	certMgr := cert.NewCertManager(dataDir)

	if !certMgr.HasCA() {
		return "CA 证书不存在，请先生成证书"
	}

	if err := cert.InstallCACert(certMgr.CACertFile(), a.logProxy); err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "permission denied") || strings.Contains(errStr, "权限") || strings.Contains(errStr, "denied") {
			return fmt.Sprintf("需要 root 权限才能安装系统 CA 证书。\n请运行: sudo %s elevate\n\n(非必需: 不安装也可正常使用，只是浏览器会提示不安全)", os.Args[0])
		}
		return fmt.Sprintf("安装 CA 证书失败: %v", err)
	}

	return ""
}

func (a *App) UninstallCACert() string {
	cert.RemoveCACert(a.logProxy)
	return ""
}

func (a *App) GetCertStatus() map[string]interface{} {
	dataDir := a.getDataDir()
	certMgr := cert.NewCertManager(dataDir)

	osName := rt.GOOS
	distro := platform.DetectDistro()
	caMethod := platform.DetectCAMethod()

	platformLabel := osName
	if distro != "" {
		platformLabel = fmt.Sprintf("%s (%s)", distro, osName)
	}

	return map[string]interface{}{
		"has_ca":           certMgr.HasCA(),
		"has_server_cert":  certMgr.HasServerCert(),
		"ca_dir":           certMgr.CADir(),
		"ca_cert_file":     certMgr.CACertFile(),
		"server_cert_file": certMgr.SrvCertFile(),
		"platform":         osName,
		"distro":           distro,
		"platform_label":   platformLabel,
		"ca_method":        caMethod,
	}
}

func (a *App) GenerateCACert() string {
	dataDir := a.getDataDir()
	certMgr := cert.NewCertManager(dataDir)

	if err := certMgr.GenerateCA(a.logProxy); err != nil {
		return fmt.Sprintf("生成 CA 证书失败: %v", err)
	}
	return ""
}

func (a *App) GenerateServerCerts() string {
	dataDir := a.getDataDir()
	certMgr := cert.NewCertManager(dataDir)

	if !certMgr.HasCA() {
		return "CA 证书不存在，请先生成 CA"
	}

	var errs []string
	if err := certMgr.GenerateServerCert(a.logProxy); err != nil {
		errs = append(errs, fmt.Sprintf("服务器证书: %v", err))
	}
	if err := certMgr.GenerateOpenRouterCert(a.logProxy); err != nil {
		errs = append(errs, fmt.Sprintf("OpenRouter证书: %v", err))
	}

	if len(errs) > 0 {
		return strings.Join(errs, "; ")
	}
	return ""
}

func (a *App) RegenerateAllCerts() string {
	dataDir := a.getDataDir()
	certMgr := cert.NewCertManager(dataDir)

	a.logProxy("正在删除旧证书...")
	if err := certMgr.RemoveLocalArtifacts(a.logProxy); err != nil {
		return fmt.Sprintf("删除旧证书失败: %v", err)
	}

	a.logProxy("正在生成新 CA 证书...")
	if err := certMgr.GenerateCA(a.logProxy); err != nil {
		return fmt.Sprintf("生成 CA 证书失败: %v", err)
	}

	a.logProxy("正在生成服务器证书...")
	if err := certMgr.GenerateServerCert(a.logProxy); err != nil {
		return fmt.Sprintf("生成服务器证书失败: %v", err)
	}
	if err := certMgr.GenerateOpenRouterCert(a.logProxy); err != nil {
		return fmt.Sprintf("生成 OpenRouter 证书失败: %v", err)
	}

	a.logProxy("所有证书重新生成完成")
	return ""
}

func (a *App) RemoveLocalCerts() string {
	dataDir := a.getDataDir()
	certMgr := cert.NewCertManager(dataDir)

	if err := certMgr.RemoveLocalArtifacts(a.logProxy); err != nil {
		return fmt.Sprintf("删除本地证书失败: %v", err)
	}
	return ""
}

func (a *App) OpenFileDialog() string {
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "选择配置文件",
		Filters: []wruntime.FileFilter{
			{DisplayName: "配置文件", Pattern: "*.toml"},
			{DisplayName: "JSON 文件", Pattern: "*.json"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
	if err != nil {
		return ""
	}
	return path
}

func (a *App) OpenDirectoryDialog() string {
	path, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "选择目录",
	})
	if err != nil {
		return ""
	}
	return path
}

func (a *App) buildConfigContent(data ConfigData) string {
	var sb strings.Builder

	sb.WriteString("# OpenHijack Configuration\n")
	sb.WriteString("# Generated by OpenHijack GUI\n\n")
	sb.WriteString("mapped_model_id = \"" + data.MappedModelID + "\"\n")
	sb.WriteString("auth_key = \"" + data.AuthKey + "\"\n")
	sb.WriteString("current_config_index = 0\n\n")

	for _, group := range data.ConfigGroups {
		sb.WriteString("[[config_groups]]\n")
		sb.WriteString("name = \"" + group.Name + "\"\n")
		sb.WriteString("provider = \"" + group.Provider + "\"\n")
		sb.WriteString("api_url = \"" + group.APIURL + "\"\n")
		sb.WriteString("model_id = \"" + group.ModelID + "\"\n")
		sb.WriteString("api_key = \"" + group.APIKey + "\"\n")
		sb.WriteString("middle_route = \"" + group.MiddleRoute + "\"\n\n")
	}

	return sb.String()
}

func (a *App) resolveConfigPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			rest := path[1:]
			rest = strings.TrimLeft(rest, string(filepath.Separator))
			rest = strings.ReplaceAll(rest, "/", string(filepath.Separator))
			rest = strings.ReplaceAll(rest, "\\", string(filepath.Separator))
			return filepath.Clean(filepath.Join(home, rest))
		}
	}

	path = strings.ReplaceAll(path, "/", string(filepath.Separator))
	path = strings.ReplaceAll(path, "\\", string(filepath.Separator))

	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	return filepath.Clean(filepath.Join(a.getConfigDir(), path))
}

func (a *App) getConfigDir() string {
	home := a.resolveHomeDir()
	if home == "" {
		return ".openhijack"
	}
	return filepath.Join(home, ".config", "openhijack")
}

func (a *App) getDataDir() string {
	dataDir, err := platform.GetDataDir()
	if err != nil {
		return ".openhijack"
	}
	return dataDir
}

func (a *App) prepareConfigForPrivilegedMode(originalPath string) string {
	if !platform.IsPrivileged() {
		return originalPath
	}

	adminConfigDir, err := platform.GetConfigDir()
	if err != nil {
		a.logProxy("警告: 无法获取管理员配置目录: %v", err)
		return originalPath
	}

	if strings.HasPrefix(filepath.Clean(originalPath), filepath.Clean(adminConfigDir)) {
		a.logProxy("配置已在管理员目录中: %s", originalPath)
		return originalPath
	}

	a.logProxy("检测到特权模式，正在复制配置文件...")
	a.logProxy("原始配置路径: %s", originalPath)
	a.logProxy("用户权限信息: uid=%d euid=%d", os.Getuid(), os.Geteuid())
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		a.logProxy("SUDO_USER: %s", sudoUser)
	}

	if err := platform.EnsureDir(adminConfigDir, 0700); err != nil {
		a.logProxy("错误: 创建管理员配置目录失败: %v", err)
		return originalPath
	}

	rootConfigFile := filepath.Join(adminConfigDir, "config.toml")

	srcData, err := os.ReadFile(originalPath)
	if err != nil {
		a.logProxy("错误: 读取原始配置文件失败: %v", err)
		return originalPath
	}

	if err := os.WriteFile(rootConfigFile, srcData, 0600); err != nil {
		a.logProxy("错误: 写入管理员配置文件失败: %v", err)
		return originalPath
	}

	a.logProxy("✓ 配置文件已复制到管理员目录")
	a.logProxy("  原始位置: %s", originalPath)
	a.logProxy("  新位置:   %s", rootConfigFile)
	a.lastConfig = rootConfigFile

	return rootConfigFile
}

func (a *App) resolveHomeDir() string {
	var euid int
	if platform.IsPrivileged() {
		euid = 0
	} else {
		euid = -1
	}

	sudoUser := os.Getenv("SUDO_USER")
	home, err := platform.ResolveHomeDir(euid, sudoUser)
	if err != nil {
		return ""
	}
	return home
}

type ProviderInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	DefaultURL   string   `json:"default_url"`
	DefaultRoute string   `json:"default_route"`
	Models       []string `json:"models"`
	APIKeyHint   string   `json:"api_key_hint"`
}

type ImportResult struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Configs []string `json:"configs"`
}

func (a *App) GetSupportedProviders() []ProviderInfo {
	return []ProviderInfo{
		{
			ID:           "openai_chat_completion",
			Name:         "OpenAI 兼容 API",
			DefaultURL:   "https://api.openai.com",
			DefaultRoute: "/v1",
			Models:       []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo", "o1", "o1-mini"},
			APIKeyHint:   "sk-...",
		},
		{
			ID:           "openrouter",
			Name:         "OpenRouter 聚合 API",
			DefaultURL:   "https://openrouter.ai/api",
			DefaultRoute: "/v1",
			Models:       []string{"anthropic/claude-sonnet-4", "openai/gpt-4o", "google/gemini-2.5-pro", "meta-llama/llama-4-maverick"},
			APIKeyHint:   "sk-or-v1-...",
		},
		{
			ID:           "anthropic",
			Name:         "Anthropic Claude",
			DefaultURL:   "https://api.anthropic.com",
			DefaultRoute: "/v1",
			Models:       []string{"claude-sonnet-4-20250514", "claude-opus-4-20250514", "claude-haiku-3-20240307"},
			APIKeyHint:   "sk-ant-...",
		},
		{
			ID:           "gemini",
			Name:         "Google Gemini",
			DefaultURL:   "https://generativelanguage.googleapis.com",
			DefaultRoute: "/v1beta",
			Models:       []string{"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.0-flash"},
			APIKeyHint:   "AIza...",
		},
		{
			ID:           "openai_response",
			Name:         "OpenAI Response API",
			DefaultURL:   "https://api.openai.com",
			DefaultRoute: "/v1",
			Models:       []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo"},
			APIKeyHint:   "sk-...",
		},
	}
}

func (a *App) GetProviderDefaults(providerID string) map[string]string {
	if defaults, ok := providerDefaults[providerID]; ok {
		return defaults
	}
	return map[string]string{
		"api_url":      "",
		"middle_route": "/v1",
	}
}

func (a *App) ExportConfig(configPath string) string {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "配置文件不存在"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Sprintf("加载配置失败: %v", err)
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Sprintf("序列化配置失败: %v", err)
	}

	return string(data)
}

func mkdirAll(dir string) error {
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		if strings.Contains(err.Error(), "Access is denied") || strings.Contains(err.Error(), "拒绝访问") {
			info, statErr := os.Stat(dir)
			if statErr == nil && info.IsDir() {
				return nil
			}
		}
		return err
	}
	return nil
}

func (a *App) ImportConfig(configText string, savePath string) string {
	if configText == "" {
		return "配置内容不能为空"
	}

	if savePath == "" {
		return "请指定保存路径"
	}

	savePath = a.resolveConfigPath(savePath)

	var cfg config.Config
	if err := toml.Unmarshal([]byte(configText), &cfg); err != nil {
		return fmt.Sprintf("解析配置失败: %v", err)
	}

	if len(cfg.ConfigGroups) == 0 {
		return "配置文件中没有 config_groups"
	}

	dir := filepath.Dir(savePath)
	if err := mkdirAll(dir); err != nil {
		return fmt.Sprintf("创建目录失败: %v", err)
	}

	if err := os.WriteFile(savePath, []byte(configText), 0600); err != nil {
		return fmt.Sprintf("保存配置文件失败: %v", err)
	}

	return ""
}

func (a *App) ImportConfigFromJSON(jsonText string, savePath string) string {
	if jsonText == "" {
		return "配置内容不能为空"
	}

	if savePath == "" {
		return "请指定保存路径"
	}

	savePath = a.resolveConfigPath(savePath)

	var importData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonText), &importData); err != nil {
		return fmt.Sprintf("解析 JSON 失败: %v", err)
	}

	cfg := a.convertJSONToConfig(importData)
	if cfg == nil {
		return "无法识别的配置格式"
	}

	tomlData, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Sprintf("转换为配置失败: %v", err)
	}

	dir := filepath.Dir(savePath)
	if err := mkdirAll(dir); err != nil {
		return fmt.Sprintf("创建目录失败: %v", err)
	}

	if err := os.WriteFile(savePath, tomlData, 0600); err != nil {
		return fmt.Sprintf("保存配置文件失败: %v", err)
	}

	return ""
}

func (a *App) convertJSONToConfig(data map[string]interface{}) *config.Config {
	cfg := &config.Config{}

	if v, ok := data["mapped_model_id"].(string); ok {
		cfg.MappedModelID = v
	}
	if v, ok := data["auth_key"].(string); ok {
		cfg.AuthKey = v
	}

	if groups, ok := data["config_groups"].([]interface{}); ok {
		for _, g := range groups {
			if gm, ok := g.(map[string]interface{}); ok {
				cg := config.ConfigGroup{}
				if v, ok := gm["name"].(string); ok {
					cg.Name = v
				}
				if v, ok := gm["provider"].(string); ok {
					cg.Provider = v
				}
				if v, ok := gm["api_url"].(string); ok {
					cg.APIURL = v
				}
				if v, ok := gm["model_id"].(string); ok {
					cg.ModelID = v
				}
				if v, ok := gm["api_key"].(string); ok {
					cg.APIKey = v
				}
				if v, ok := gm["middle_route"].(string); ok {
					cg.MiddleRoute = v
				}
				cfg.ConfigGroups = append(cfg.ConfigGroups, cg)
			}
		}
	}

	if len(cfg.ConfigGroups) == 0 {
		return nil
	}

	return cfg
}

func (a *App) ImportConfigFromFile(filePath string, savePath string) string {
	if filePath == "" {
		return "请选择配置文件"
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Sprintf("读取文件失败: %v", err)
	}

	if savePath == "" {
		savePath = filepath.Join(a.getConfigDir(), filepath.Base(filePath))
	}

	if strings.HasSuffix(strings.ToLower(filePath), ".json") {
		return a.ImportConfigFromJSON(string(data), savePath)
	}

	return a.ImportConfig(string(data), savePath)
}

type ConfigFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Data string `json:"data"`
}

func (a *App) LoadConfigFile(filePath string) string {
	if filePath == "" {
		return "请指定文件路径"
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Sprintf("读取文件失败: %v", err)
	}

	return string(data)
}

func (a *App) LoadFullConfig(filePath string) ConfigData {
	if filePath == "" {
		return ConfigData{}
	}

	cfg, err := config.Load(filePath)
	if err != nil {
		a.logProxy("加载配置失败: %v", err)
		return ConfigData{Path: filePath}
	}

	groups := make([]ConfigGroupData, 0, len(cfg.ConfigGroups))
	for _, g := range cfg.ConfigGroups {
		groups = append(groups, ConfigGroupData{
			Name:        g.Name,
			Provider:    g.Provider,
			APIURL:      g.APIURL,
			ModelID:     g.ModelID,
			APIKey:      g.APIKey,
			MiddleRoute: g.MiddleRoute,
		})
	}

	return ConfigData{
		Path:          filePath,
		MappedModelID: cfg.MappedModelID,
		AuthKey:       cfg.AuthKey,
		ConfigGroups:  groups,
	}
}
