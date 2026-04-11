package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"syscall"

	"openhijack/internal/cert"
	"openhijack/internal/hosts"
	"openhijack/internal/proxy"
)

const (
	defaultListenHost = ""
	defaultListenPort = 443
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "elevate" {
		runElevate()
		return
	}

	serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
	host := serveCmd.String("host", defaultListenHost, "监听地址")
	port := serveCmd.Int("port", defaultListenPort, "监听端口")
	debug := serveCmd.Bool("debug", false, "调试模式")
	disableSSLStrict := serveCmd.Bool("disable-ssl-strict-mode", false, "禁用 SSL 严格模式")
	forceStream := serveCmd.Bool("force-stream", false, "强制使用流模式")
	httpMode := serveCmd.Bool("http", false, "使用纯 HTTP 模式 (不使用 TLS)")
	noManage := serveCmd.Bool("no-manage", false, "不自动管理证书和 hosts (手动指定证书时使用)")
	configPath := serveCmd.String("config", "", "配置文件路径")

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
	case "paths":
		pathsCmd.Parse(os.Args[2:])
		printPaths()
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
	fmt.Fprintf(os.Stderr, "  openhijack serve [选项]    启动代理服务器 (默认 HTTPS:443)\n")
	fmt.Fprintf(os.Stderr, "  openhijack paths           显示数据路径\n")
	fmt.Fprintf(os.Stderr, "  openhijack elevate         权限提升并启动 (sudo)\n\n")
	fmt.Fprintf(os.Stderr, "serve 选项:\n")
	fmt.Fprintf(os.Stderr, "  --host string                监听地址 (默认: 所有接口)\n")
	fmt.Fprintf(os.Stderr, "  --port int                   监听端口 (默认: %d)\n", defaultListenPort)
	fmt.Fprintf(os.Stderr, "  --config string              配置文件路径\n")
	fmt.Fprintf(os.Stderr, "  --debug                      调试模式\n")
	fmt.Fprintf(os.Stderr, "  --http                       使用纯 HTTP 模式 (不使用 TLS)\n")
	fmt.Fprintf(os.Stderr, "  --no-manage                  不自动管理证书和 hosts\n")
	fmt.Fprintf(os.Stderr, "  --disable-ssl-strict-mode    禁用 SSL 严格模式\n")
	fmt.Fprintf(os.Stderr, "  --force-stream               强制使用流模式\n")
}

func getConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法获取用户主目录: %v\n", err)
		os.Exit(1)
	}
	return filepath.Join(home, ".config", "openhijack")
}

func getDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法获取用户主目录: %v\n", err)
		os.Exit(1)
	}
	return filepath.Join(home, ".local", "share", "openhijack")
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
		fmt.Fprintf(os.Stderr, "配置文件不存在: %s\n", configPath)
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
			logger.Printf("清理: 移除 hosts 条目...")
			if err := hostsMgr.RemoveEntry(logger.Printf); err != nil {
				logger.Printf("清理 hosts 失败: %v", err)
			}
		}
	} else if useTLS && noManage {
		certMgr := cert.NewCertManager(dataDir)
		tlsCertFile, tlsKeyFile = certMgr.TLSCert()
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
		CleanupFn:        cleanupFn,
	}

	server, err := proxy.NewProxyServer(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建代理服务器失败: %v\n", err)
		os.Exit(1)
	}

	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "启动代理服务器失败: %v\n", err)
		if cleanupFn != nil {
			cleanupFn()
		}
		os.Exit(1)
	}

	server.Wait()
}

func runElevate() {
	if os.Geteuid() == 0 {
		scriptUser := resolveScriptUser()
		userHome := resolveUserHome(scriptUser)

		configSrcDefault := filepath.Join(userHome, ".config", "openhijack", "config.toml")
		configSrc := envOrDefault("OPENHIJACK_CONFIG", configSrcDefault)

		if _, err := os.Stat(configSrc); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "配置文件不存在: %s\n", configSrc)
			os.Exit(1)
		}

		rootConfigDir := "/root/.config/openhijack"
		if err := os.MkdirAll(rootConfigDir, 0700); err != nil {
			fmt.Fprintf(os.Stderr, "创建配置目录失败: %v\n", err)
			os.Exit(1)
		}

		rootConfigFile := filepath.Join(rootConfigDir, "config.toml")
		srcData, err := os.ReadFile(configSrc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取配置文件失败: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(rootConfigFile, srcData, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "写入配置文件失败: %v\n", err)
			os.Exit(1)
		}

		userDataDir := filepath.Join(userHome, ".local", "share", "openhijack")
		certMgr := cert.NewCertManager(userDataDir)
		logf := func(format string, args ...interface{}) { fmt.Printf(format, args...) }

		if !certMgr.HasCA() {
			logf("CA 证书不存在，开始生成...\n")
			if err := certMgr.GenerateCA(logf); err != nil {
				fmt.Fprintf(os.Stderr, "生成 CA 证书失败: %v\n", err)
				os.Exit(1)
			}
		}

		if !certMgr.HasServerCert() {
			logf("服务器证书不存在，开始生成...\n")
			if err := certMgr.GenerateServerCert(logf); err != nil {
				fmt.Fprintf(os.Stderr, "生成服务器证书失败: %v\n", err)
				os.Exit(1)
			}
		}

		if err := cert.InstallCACert(certMgr.CACertFile(), logf); err != nil {
			fmt.Fprintf(os.Stderr, "安装 CA 证书失败: %v\n", err)
			os.Exit(1)
		}

		hostsMgr := hosts.NewHostsManager(userDataDir)
		if err := hostsMgr.AddEntry(logf); err != nil {
			fmt.Fprintf(os.Stderr, "修改 hosts 文件失败: %v\n", err)
			os.Exit(1)
		}

		args := []string{"serve", "--config", rootConfigFile}
		args = append(args, os.Args[2:]...)

		bin, err := exec.LookPath("openhijack")
		if err != nil {
			self, _ := os.Executable()
			bin = self
		}

		env := os.Environ()
		env = append(env, fmt.Sprintf("OPENHIJACK_CONFIG=%s", configSrc))

		syscall.Exec(bin, append([]string{"openhijack"}, args...), env)
		return
	}

	selfPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法获取可执行文件路径: %v\n", err)
		os.Exit(1)
	}

	args := []string{"--preserve-env=OPENHIJACK_CONFIG", selfPath, "elevate"}
	args = append(args, os.Args[2:]...)

	cmd := exec.Command("sudo", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	cmd.Run()
}

func resolveScriptUser() string {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		return sudoUser
	}
	if currentUser, err := user.Current(); err == nil {
		return currentUser.Username
	}
	return "snemc"
}

func resolveUserHome(username string) string {
	if u, err := user.Lookup(username); err == nil {
		return u.HomeDir
	}
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	return "/home/" + username
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
	fmt.Printf("Hosts 备份:   %s\n", hostsMgr.BackupPath())
}
