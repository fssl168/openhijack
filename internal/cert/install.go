package cert

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const caCertFileName = "openhijack-ca.crt"

func InstallCACert(caCertPath string, logf func(string, ...interface{})) error {
	switch runtime.GOOS {
	case "linux":
		return installCALinux(caCertPath, logf)
	case "darwin":
		return installCADarwin(caCertPath, logf)
	case "windows":
		return installCAWindows(caCertPath, logf)
	case "freebsd":
		return installCAFreeBSD(caCertPath, logf)
	default:
		return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}
}

func RemoveCACert(logf func(string, ...interface{})) {
	switch runtime.GOOS {
	case "linux":
		removeCALinux(logf)
	case "darwin":
		removeCADarwin(logf)
	case "windows":
		removeCAWindows(logf)
	case "freebsd":
		removeCAFreeBSD(logf)
	default:
		logf("不支持的操作系统: %s", runtime.GOOS)
	}
}

func installCALinux(caCertPath string, logf func(string, ...interface{})) error {
	src, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("读取 CA 证书失败: %w", err)
	}

	methods := []struct {
		name      string
		detectCmd string
		anchorDir string
		runCmd    []string
		success   string
	}{
		{
			name:      "update-ca-certificates",
			detectCmd: "update-ca-certificates",
			anchorDir: "/usr/local/share/ca-certificates/",
			runCmd:    []string{"update-ca-certificates"},
			success:   "Debian / Ubuntu / Alpine Linux",
		},
		{
			name:      "update-ca-trust",
			detectCmd: "update-ca-trust",
			anchorDir: "/etc/pki/ca-trust/source/anchors/",
			runCmd:    []string{"update-ca-trust", "extract"},
			success:   "RHEL / CentOS / openSUSE",
		},
		{
			name:      "trust (p11-kit)",
			detectCmd: "trust",
			anchorDir: "/etc/ca-certificates/trust-source/anchors/",
			runCmd:    []string{"trust", "extract-compat"},
			success:   "Arch Linux / Fedora",
		},
	}

	var lastErr error
	for _, m := range methods {
		if _, err := exec.LookPath(m.detectCmd); err != nil {
			continue
		}

		if err := os.MkdirAll(m.anchorDir, 0755); err != nil {
			lastErr = fmt.Errorf("创建 %s anchors 目录失败: %w", m.name, err)
			continue
		}
		dst := filepath.Join(m.anchorDir, caCertFileName)
		if err := os.WriteFile(dst, src, 0644); err != nil {
			lastErr = fmt.Errorf("复制 CA 证书到 %s 失败: %w", dst, err)
			continue
		}
		logf("CA 证书已复制到 %s (%s)", dst, m.name)

		logf("运行 %s...", strings.Join(m.runCmd, " "))
		cmd := exec.Command(m.runCmd[0], m.runCmd[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logf("%s 失败: %v，尝试下一个方法...", m.name, err)
			os.Remove(dst)
			lastErr = fmt.Errorf("%s 失败: %w", m.name, err)
			continue
		}

		logf("CA 证书安装成功 (%s)", m.success)
		return nil
	}

	if lastErr != nil {
		return lastErr
	}
	return buildLinuxInstallError()
}

func buildLinuxInstallError() error {
	var hint strings.Builder

	hint.WriteString("未找到可用的 CA 证书管理工具。\n\n")
	hint.WriteString("请根据你的发行版安装相应的包：\n\n")

	hint.WriteString("Debian / Ubuntu:\n")
	hint.WriteString("  sudo apt-get install ca-certificates\n\n")

	hint.WriteString("RHEL / CentOS / Fedora:\n")
	hint.WriteString("  sudo yum install ca-certificates\n")
	hint.WriteString("  # 或\n")
	hint.WriteString("  sudo dnf install ca-certificates\n\n")

	hint.WriteString("Arch Linux:\n")
	hint.WriteString("  sudo pacman -S trust\n\n")

	hint.WriteString("openSUSE:\n")
	hint.WriteString("  sudo zypper install ca-certificates\n\n")

	hint.WriteString("Alpine Linux:\n")
	hint.WriteString("  sudo apk add ca-certificates\n")
	hint.WriteString("  update-ca-certificates\n\n")

	hint.WriteString("或者手动将 CA 证书添加到系统信任库。\n")

	return errors.New(hint.String())
}

func removeCALinux(logf func(string, ...interface{})) {
	candidates := []struct {
		path      string
		refreshCmd string
	}{
		{
			path:      "/usr/local/share/ca-certificates/" + caCertFileName,
			refreshCmd: "update-ca-certificates",
		},
		{
			path:      "/etc/pki/ca-trust/source/anchors/" + caCertFileName,
			refreshCmd: "update-ca-trust",
		},
		{
			path:      "/etc/ca-certificates/trust-source/anchors/" + caCertFileName,
			refreshCmd: "trust",
		},
	}

	removed := false
	var refreshMethod string

	for _, c := range candidates {
		if _, err := os.Stat(c.path); err == nil {
			if err := os.Remove(c.path); err != nil {
				logf("移除 %s 失败: %v", c.path, err)
			} else {
				logf("已移除 %s", c.path)
				removed = true
				refreshMethod = c.refreshCmd
				break
			}
		}
	}

	if !removed {
		logf("系统 CA 证书不存在，跳过移除")
		return
	}

	switch refreshMethod {
	case "trust":
		if path, err := exec.LookPath("trust"); err == nil {
			cmd := exec.Command(path, "extract-compat")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				logf("trust extract-compat 失败，尝试 update-ca-certificates...")
				if path2, err2 := exec.LookPath("update-ca-certificates"); err2 == nil {
					exec.Command(path2).Run()
				}
			}
		}
	case "update-ca-trust":
		if path, err := exec.LookPath("update-ca-trust"); err == nil {
			cmd := exec.Command(path, "extract")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
		}
	case "update-ca-certificates":
		if path, err := exec.LookPath("update-ca-certificates"); err == nil {
			cmd := exec.Command(path)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
		}
	}

	logf("CA 证书已从系统移除")
}

func installCADarwin(caCertPath string, logf func(string, ...interface{})) error {
	cmd := exec.Command("security", "add-trusted-cert", "-k", "/Library/Keychains/System.keychain", caCertPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("security add-trusted-cert 失败: %w", err)
	}
	logf("CA 证书已安装到 macOS 系统钥匙串")
	return nil
}

func removeCADarwin(logf func(string, ...interface{})) {
	cmd := exec.Command("security", "remove-trusted-cert", "-d", CACommonName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logf("移除 macOS 信任证书失败 (可能已不存在): %v", err)
	}

	cmd = exec.Command("security", "delete-certificate", "-c", CACommonName, "/Library/Keychains/System.keychain")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logf("从钥匙串删除证书失败 (可能已不存在): %v", err)
	}
	logf("CA 证书已从 macOS 系统移除")
}

func installCAWindows(caCertPath string, logf func(string, ...interface{})) error {
	cmd := exec.Command("certutil", "-addstore", "-f", "ROOT", caCertPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("certutil 安装 CA 失败: %w", err)
	}
	logf("CA 证书已安装到 Windows 受信任根证书存储")
	return nil
}

func removeCAWindows(logf func(string, ...interface{})) {
	cmd := exec.Command("certutil", "-delstore", "ROOT", CACommonName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logf("从 Windows 根证书存储移除失败 (可能已不存在): %v", err)
	}
	logf("CA 证书已从 Windows 系统移除")
}

func installCAFreeBSD(caCertPath string, logf func(string, ...interface{})) error {
	src, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("读取 CA 证书失败: %w", err)
	}

	dstDir := "/usr/local/share/certs/"
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("创建 CA 目录失败: %w", err)
	}
	dst := filepath.Join(dstDir, caCertFileName)
	if err := os.WriteFile(dst, src, 0644); err != nil {
		return fmt.Errorf("复制 CA 证书到 %s 失败: %w", dst, err)
	}
	logf("CA 证书已复制到 %s", dst)

	logf("运行 certctl rehash...")
	cmd := exec.Command("certctl", "rehash")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("certctl rehash 失败: %w", err)
	}
	logf("CA 证书安装成功 (FreeBSD)")
	return nil
}

func removeCAFreeBSD(logf func(string, ...interface{})) {
	path := "/usr/local/share/certs/" + caCertFileName
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			logf("移除 %s 失败: %v", path, err)
		} else {
			logf("已移除 %s", path)
		}
	}

	cmd := exec.Command("certctl", "rehash")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	logf("CA 证书已从 FreeBSD 系统移除")
}
