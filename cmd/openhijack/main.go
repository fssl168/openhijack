package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"openhijack/internal/cert"
	"openhijack/internal/hosts"
	"openhijack/internal/platform"
	"openhijack/internal/proxy"
)

const (
	defaultListenHost = ""
	defaultListenPort = 443
	fallbackPort      = 8443
)

const defaultConfigTemplate = `# OpenHijack 配置模板
# 1. 修改下方的上游地址、模型和密钥
# 2. 客户端连接本地代理时使用 mapped_model_id 与 auth_key
# 3. 修改完成后运行 ./openhijack elevate

mapped_model_id = "my-model"
auth_key = "%s"
current_config_index = 0

[[config_groups]]
name = "default"
provider = "openai_chat_completion"
api_url = "https://your-upstream-provider.example.com"
model_id = "your-upstream-model"
api_key = "your-upstream-api-key"
middle_route = "/v1"
`

func main() {
	if len(os.Args) > 1 && os.Args[1] == "elevate" {
		runElevate()
		return
	}

	serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
	host := serveCmd.String("host", defaultListenHost, "监听地址")
	port := serveCmd.Int("port", defaultListenPort, "监听端口")
	debug := serveCmd.Bool("debug", false, "调试模式")
	disableSSLStrict := serveCmd.Bool("disable-ssl-strict-mode", false, "禁用上游 TLS 证书校验")
	forceStream := serveCmd.Bool("force-stream", false, "强制使用流模式")
	httpMode := serveCmd.Bool("http", false, "使用纯 HTTP 模式 (不使用 TLS)")
	noManage := serveCmd.Bool("no-manage", false, "不自动管理证书和 hosts (手动指定证书时使用)")
	configPath := serveCmd.String("config", "", "配置文件路径")

	initCmd := flag.NewFlagSet("init", flag.ExitOnError)
	initConfigPath := initCmd.String("config", "", "配置文件路径")
	initForce := initCmd.Bool("force", false, "覆盖已存在的配置文件")

	cleanupCmd := flag.NewFlagSet("cleanup", flag.ExitOnError)

	pathsCmd := flag.NewFlagSet("paths", flag.ExitOnError)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		serveCmd.Parse(os.Args[2:])
		configSrc := resolveConfigPath(*configPath)
		runServe(configSrc, *host, *port, *debug, *disableSSLStrict, *forceStream, !*httpMode, *noManage)
	case "init":
		initCmd.Parse(os.Args[2:])
		runInit(resolveConfigPath(*initConfigPath), *initForce)
	case "cleanup", "uninstall":
		cleanupCmd.Parse(os.Args[2:])
		runCleanup()
	case "paths":
		pathsCmd.Parse(os.Args[2:])
		printPaths()
	case "doctor":
		runDoctor()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "openhijack - 本地 HTTPS 代理服务器\n\n")
	fmt.Fprintf(os.Stderr, "用法:\n")
	fmt.Fprintf(os.Stderr, "  openhijack init [选项]     生成配置模板\n")
	fmt.Fprintf(os.Stderr, "  openhijack serve [选项]    启动代理服务器 (默认 HTTPS:443)\n")
	fmt.Fprintf(os.Stderr, "  openhijack cleanup         移除 hosts、系统 CA 和本地证书\n")
	fmt.Fprintf(os.Stderr, "  openhijack paths           显示数据路径\n")
	fmt.Fprintf(os.Stderr, "  openhijack doctor          健康检查报告\n")
	fmt.Fprintf(os.Stderr, "  openhijack elevate         权限提升并启动 (sudo)\n")
	fmt.Fprintf(os.Stderr, "  openhijack install         Linux 安装脚本 (需 sudo)\n\n")
	fmt.Fprintf(os.Stderr, "init 选项:\n")
	fmt.Fprintf(os.Stderr, "  --config string              配置文件路径\n")
	fmt.Fprintf(os.Stderr, "  --force                      覆盖已存在的配置文件\n\n")
	fmt.Fprintf(os.Stderr, "serve 选项:\n")
	fmt.Fprintf(os.Stderr, "  --host string                监听地址 (默认: 所有接口)\n")
	fmt.Fprintf(os.Stderr, "  --port int                   监听端口 (默认: %d，无权限时自动降级到 %d)\n", defaultListenPort, fallbackPort)
	fmt.Fprintf(os.Stderr, "  --config string              配置文件路径\n")
	fmt.Fprintf(os.Stderr, "  --debug                      调试模式\n")
	fmt.Fprintf(os.Stderr, "  --http                       使用纯 HTTP 模式 (不使用 TLS)\n")
	fmt.Fprintf(os.Stderr, "  --no-manage                  不自动管理证书和 hosts\n")
	fmt.Fprintf(os.Stderr, "  --disable-ssl-strict-mode    禁用上游 TLS 证书校验\n")
	fmt.Fprintf(os.Stderr, "  --force-stream               强制使用流模式\n\n")
	fmt.Fprintf(os.Stderr, "Linux 权限提示:\n")
	fmt.Fprintf(os.Stderr, "  端口 <1024 需要 root 或 cap_net_bind_service\n")
	fmt.Fprintf(os.Stderr, "  安装: sudo bash scripts/install.sh\n")
	fmt.Fprintf(os.Stderr, "  卸载: sudo bash scripts/uninstall.sh\n")
}

func generateAuthKey() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func createDefaultConfig(configPath string, force bool) (bool, error) {
	if !force {
		if _, err := os.Stat(configPath); err == nil {
			return false, nil
		} else if !os.IsNotExist(err) {
			return false, err
		}
	}

	dir := filepath.Dir(configPath)
	if err := platform.EnsureDir(dir, 0700); err != nil {
		return false, fmt.Errorf("创建配置目录失败: %w", err)
	}

	authKey, err := generateAuthKey()
	if err != nil {
		return false, fmt.Errorf("生成默认鉴权密钥失败: %w", err)
	}

	content := fmt.Sprintf(defaultConfigTemplate, authKey)
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		return false, fmt.Errorf("写入配置文件失败: %w", err)
	}

	return true, nil
}

func runInit(configPath string, force bool) {
	created, err := createDefaultConfig(configPath, force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化配置文件失败: %v\n", err)
		os.Exit(1)
	}

	if created {
		fmt.Printf("已创建配置文件: %s\n", configPath)
	} else {
		fmt.Printf("配置文件已存在: %s\n", configPath)
	}
	fmt.Println("请先修改以下字段后再启动:")
	fmt.Println("  - config_groups[0].api_url")
	fmt.Println("  - config_groups[0].model_id")
	fmt.Println("  - config_groups[0].api_key")
	fmt.Println("  - mapped_model_id / auth_key")
	fmt.Println("修改完成后可运行: ./openhijack elevate")
}

func runtimeHomeDir() string {
	var euid int
	if platform.IsPrivileged() {
		euid = 0
	} else {
		euid = -1
	}

	sudoUser := os.Getenv("SUDO_USER")
	home, err := platform.ResolveHomeDir(euid, sudoUser)
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法获取用户主目录: %v\n", err)
		os.Exit(1)
	}
	return home
}

func getConfigDir() string {
	configDir, err := platform.GetConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法获取配置目录: %v\n", err)
		os.Exit(1)
	}
	return configDir
}

func getDataDir() string {
	dataDir, err := platform.GetDataDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法获取数据目录: %v\n", err)
		os.Exit(1)
	}
	return dataDir
}

func resolveConfigPath(configPath string) string {
	if configPath != "" {
		return configPath
	}
	if envPath := os.Getenv("OPENHIJACK_CONFIG"); envPath != "" {
		return envPath
	}
	return filepath.Join(getConfigDir(), "config.toml")
}

func runServe(configPath string, host string, port int, debug bool, disableSSLStrict bool, forceStream bool, useTLS bool, noManage bool) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "配置文件不存在: %s\n请先运行 `openhijack init` 生成模板并修改上游配置。\n", configPath)
		os.Exit(1)
	}

	logger := log.New(os.Stderr, "[openhijack] ", log.LstdFlags|log.Lmicroseconds)
	dataDir := getDataDir()

	var tlsCertFile, tlsKeyFile string
	var cleanupFn func()

	if useTLS && !noManage {
		certMgr := cert.NewCertManager(dataDir)
		hostsMgr := hosts.NewHostsManager(dataDir)

		if !certMgr.HasCA() {
			logger.Printf("CA 证书不存在，开始生成...")
			if err := certMgr.GenerateCA(logger.Printf); err != nil {
				fmt.Fprintf(os.Stderr, "生成 CA 证书失败: %v\n", err)
				os.Exit(1)
			}
		} else {
			logger.Printf("CA 证书已存在")
		}

		if !certMgr.HasServerCert() {
			logger.Printf("服务器证书不存在，开始生成...")
			if err := certMgr.GenerateServerCert(logger.Printf); err != nil {
				fmt.Fprintf(os.Stderr, "生成服务器证书失败: %v\n", err)
				os.Exit(1)
			}
			if err := certMgr.GenerateOpenRouterCert(logger.Printf); err != nil {
				fmt.Fprintf(os.Stderr, "生成 OpenRouter 证书失败: %v\n", err)
				os.Exit(1)
			}
		} else {
			logger.Printf("服务器证书已存在")
		}

		if err := cert.InstallCACert(certMgr.CACertFile(), logger.Printf); err != nil {
			fmt.Fprintf(os.Stderr, "安装 CA 证书到系统失败: %v\n", err)
			os.Exit(1)
		}

		if err := hostsMgr.AddEntry(logger.Printf); err != nil {
			fmt.Fprintf(os.Stderr, "修改 hosts 文件失败: %v\n", err)
			os.Exit(1)
		}

		tlsCertFile, tlsKeyFile = certMgr.TLSCert()

		cleanupFn = func() {
			cleanupRuntimeState(dataDir, logger.Printf)
		}
	} else if useTLS && noManage {
		certMgr := cert.NewCertManager(dataDir)
		tlsCertFile, tlsKeyFile = certMgr.TLSCert()
	}

	var extraTLSCerts map[string]string
	if useTLS {
		certMgr := cert.NewCertManager(dataDir)
		extraTLSCerts = certMgr.ExtraTLSCerts()
	}

	opts := proxy.ServeOptions{
		ConfigPath:       configPath,
		Host:             host,
		Port:             port,
		UseTLS:           useTLS,
		DebugMode:        debug,
		DisableSSLStrict: disableSSLStrict,
		ForceStream:      forceStream,
		TLSCertFile:      tlsCertFile,
		TLSKeyFile:       tlsKeyFile,
		LogCallback: func(msg string) {
			logger.Print(msg)
		},
		ExtraTLSCerts: extraTLSCerts,
		CleanupFn:     cleanupFn,
	}

	server, err := proxy.NewProxyServer(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建代理服务器失败: %v\n", err)
		if cleanupFn != nil {
			cleanupFn()
		}
		os.Exit(1)
	}

	if err := server.Start(); err != nil {
		if isPrivilegedPortError(err) && port < 1024 && !platform.IsPrivileged() && port != fallbackPort {
			logger.Printf("端口 %d 绑定失败 (需要特权权限)，自动降级到端口 %d", port, fallbackPort)
			if cleanupFn != nil {
				cleanupFn()
			}
			runServe(configPath, host, fallbackPort, debug, disableSSLStrict, forceStream, useTLS, noManage)
			return
		}
		fmt.Fprintf(os.Stderr, "启动代理服务器失败: %v\n", err)
		if cleanupFn != nil {
			cleanupFn()
		}
		os.Exit(1)
	}

	server.Wait()
}

func isPrivilegedPortError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "bind: permission") ||
		strings.Contains(errStr, "address already in use") ||
		strings.Contains(errStr, "socket operation on non-socket") ||
		strings.Contains(errStr, "EACCES")
}

func cleanupRuntimeState(dataDir string, logf func(string, ...interface{})) {
	hostsMgr := hosts.NewHostsManager(dataDir)

	logf("清理: 移除 hosts 条目...")
	if err := hostsMgr.RemoveEntry(logf); err != nil {
		logf("清理 hosts 失败: %v", err)
	}

	logf("清理: 移除系统 CA 信任...")
	cert.RemoveCACert(logf)
}

func cleanupInstallation(dataDir string, logf func(string, ...interface{})) error {
	cleanupRuntimeState(dataDir, logf)

	certMgr := cert.NewCertManager(dataDir)
	hostsMgr := hosts.NewHostsManager(dataDir)

	var errs []error
	if err := certMgr.RemoveLocalArtifacts(logf); err != nil {
		errs = append(errs, fmt.Errorf("移除本地证书失败: %w", err))
	}
	if err := hostsMgr.RemoveBackup(logf); err != nil {
		errs = append(errs, fmt.Errorf("移除 hosts 备份失败: %w", err))
	}
	if err := os.Remove(dataDir); err != nil {
		if !os.IsNotExist(err) && !errors.Is(err, syscall.ENOTEMPTY) {
			errs = append(errs, fmt.Errorf("移除数据目录失败: %w", err))
		}
	} else {
		logf("已移除数据目录: %s", dataDir)
	}
	return errors.Join(errs...)
}

func runCleanup() {
	if !platform.IsPrivileged() {
		args := []string{"cleanup"}
		args = append(args, os.Args[2:]...)

		if err := platform.Elevate(args, os.Environ()); err != nil {
			fmt.Fprintf(os.Stderr, "权限提升失败: %v\n", err)
			os.Exit(1)
		}
		return
	}

	logger := log.New(os.Stderr, "[openhijack] ", log.LstdFlags|log.Lmicroseconds)
	if err := cleanupInstallation(getDataDir(), logger.Printf); err != nil {
		fmt.Fprintf(os.Stderr, "清理失败: %v\n", err)
		os.Exit(1)
	}
}

func runElevate() {
	if platform.IsPrivileged() {
		configSrc := envOrDefault("OPENHIJACK_CONFIG", "")

		if configSrc == "" {
			configDir, err := platform.GetConfigDir()
			if err != nil {
				fmt.Fprintf(os.Stderr, "无法获取配置目录: %v\n", err)
				os.Exit(1)
			}
			configSrc = filepath.Join(configDir, "config.toml")
		}

		if _, err := os.Stat(configSrc); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "配置文件不存在: %s\n请先运行 `openhijack init` 生成模板并修改上游配置。\n", configSrc)
			os.Exit(1)
		}

		adminConfigDir, err := platform.GetConfigDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "无法获取管理员配置目录: %v\n", err)
			os.Exit(1)
		}

		if err := platform.EnsureDir(adminConfigDir, 0700); err != nil {
			fmt.Fprintf(os.Stderr, "创建配置目录失败: %v\n", err)
			os.Exit(1)
		}

		rootConfigFile := filepath.Join(adminConfigDir, "config.toml")
		srcData, err := os.ReadFile(configSrc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取配置文件失败: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(rootConfigFile, srcData, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "写入配置文件失败: %v\n", err)
			os.Exit(1)
		}

		host := defaultListenHost
		port := defaultListenPort
		debug := false
		disableSSLStrict := false
		forceStream := false
		useTLS := true
		noManage := false

		extraArgs := os.Args[2:]
		for i := 0; i < len(extraArgs); i++ {
			switch extraArgs[i] {
			case "--host":
				if i+1 < len(extraArgs) {
					host = extraArgs[i+1]
					i++
				}
			case "--port":
				if i+1 < len(extraArgs) {
					fmt.Sscanf(extraArgs[i+1], "%d", &port)
					i++
				}
			case "--debug":
				debug = true
			case "--disable-ssl-strict-mode":
				disableSSLStrict = true
			case "--force-stream":
				forceStream = true
			case "--http":
				useTLS = false
			case "--no-manage":
				noManage = true
			}
		}

		runServe(rootConfigFile, host, port, debug, disableSSLStrict, forceStream, useTLS, noManage)
		return
	}

	args := []string{"elevate"}
	args = append(args, os.Args[2:]...)

	env := os.Environ()
	configDir, err := platform.GetConfigDir()
	if err == nil {
		defaultConfigPath := filepath.Join(configDir, "config.toml")
		env = append(env, fmt.Sprintf("OPENHIJACK_CONFIG=%s", defaultConfigPath))
	}

	if err := platform.Elevate(args, env); err != nil {
		fmt.Fprintf(os.Stderr, "权限提升失败: %v\n", err)
		os.Exit(1)
	}
}

func resolveScriptUser() string {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		return sudoUser
	}
	if currentUser, err := user.Current(); err == nil && currentUser.Username != "" {
		return currentUser.Username
	}
	return "root"
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func printPaths() {
	configDir := getConfigDir()
	dataDir := getDataDir()
	certMgr := cert.NewCertManager(dataDir)
	hostsMgr := hosts.NewHostsManager(dataDir)
	fmt.Printf("配置目录:     %s\n", configDir)
	fmt.Printf("配置文件:     %s\n", filepath.Join(configDir, "config.toml"))
	fmt.Printf("数据目录:     %s\n", dataDir)
	fmt.Printf("CA 目录:      %s\n", certMgr.CADir())
	fmt.Printf("CA 证书:      %s\n", certMgr.CACertFile())
	fmt.Printf("CA 私钥:      %s\n", certMgr.CAKeyFile())
	fmt.Printf("服务器证书:   %s\n", certMgr.SrvCertFile())
	fmt.Printf("服务器私钥:   %s\n", certMgr.SrvKeyFile())
	fmt.Printf("Hosts 文件:   %s\n", platform.GetHostsPath())
	fmt.Printf("Hosts 备份:   %s\n", hostsMgr.BackupPath())
}
