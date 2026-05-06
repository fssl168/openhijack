package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	rt "runtime"
	"strings"
	"sync"
	"time"

	"openhijack/internal/cert"
	"openhijack/internal/config"
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
	logBuffer  *LogBuffer
	logChan    chan string
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

	go a.processLogStream()
	go a.detectAutoStart()
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

	if a.running {
		status.Uptime = time.Since(a.startTime).Round(time.Second).String()
		if cfg, err := config.Load(a.lastConfig); err == nil {
			group := cfg.CurrentGroup()
			status.Model = group.ModelID
			status.Provider = group.Provider
		}
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

	if home, err := os.UserHomeDir(); err == nil {
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

func (a *App) StartProxy(configPath string, port int) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return "代理服务已在运行中"
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Sprintf("配置文件不存在: %s", configPath)
	}

	dataDir := a.getDataDir()
	certMgr := cert.NewCertManager(dataDir)

	var tlsCertFile, tlsKeyFile string

	if !certMgr.HasCA() {
		if err := certMgr.GenerateCA(a.logProxy); err != nil {
			return fmt.Sprintf("生成 CA 证书失败: %v", err)
		}
	}

	if !certMgr.HasServerCert() {
		if err := certMgr.GenerateServerCert(a.logProxy); err != nil {
			return fmt.Sprintf("生成服务器证书失败: %v", err)
		}
		if err := certMgr.GenerateOpenRouterCert(a.logProxy); err != nil {
			return fmt.Sprintf("生成 OpenRouter 证书失败: %v", err)
		}
	}

	if err := cert.InstallCACert(certMgr.CACertFile(), a.logProxy); err != nil {
		a.logProxy(fmt.Sprintf("安装 CA 证书到系统失败 (可忽略): %v", err))
	}

	tlsCertFile, tlsKeyFile = certMgr.TLSCert()

	opts := proxy.ServeOptions{
		ConfigPath:  configPath,
		Host:        "0.0.0.0",
		Port:        port,
		UseTLS:      true,
		DebugMode:   false,
		ForceStream: false,
		TLSCertFile: tlsCertFile,
		TLSKeyFile:  tlsKeyFile,
	}

	server, err := proxy.NewProxyServer(opts)
	if err != nil {
		return fmt.Sprintf("创建代理服务器失败: %v", err)
	}

	go func() {
		if err := server.Start(); err != nil {
			a.logProxy(fmt.Sprintf("代理服务器错误: %v", err))
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

	if !a.running {
		return "代理服务未运行"
	}

	if a.server != nil {
		a.server.Stop()
		a.server = nil
	}

	a.running = false
	a.logProxy("代理服务已停止")

	return ""
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

func (a *App) InstallCACert() string {
	dataDir := a.getDataDir()
	certMgr := cert.NewCertManager(dataDir)

	if !certMgr.HasCA() {
		return "CA 证书不存在，请先生成证书"
	}

	if err := cert.InstallCACert(certMgr.CACertFile(), a.logProxy); err != nil {
		return fmt.Sprintf("安装 CA 证书失败: %v", err)
	}

	return ""
}

func (a *App) UninstallCACert() string {
	cert.RemoveCACert(a.logProxy)
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
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "openhijack")
	}
	return ".openhijack"
}

func (a *App) getDataDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "openhijack")
	}
	return ".openhijack"
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
