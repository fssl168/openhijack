package cert

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	default:
		logf("不支持的操作系统: %s", runtime.GOOS)
	}
}

func installCALinux(caCertPath string, logf func(string, ...interface{})) error {
	src, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("读取 CA 证书失败: %w", err)
	}

	if path, err := exec.LookPath("trust"); err == nil {
		anchorDir := "/etc/ca-certificates/trust-source/anchors/"
		if err := os.MkdirAll(anchorDir, 0755); err != nil {
			return fmt.Errorf("创建 anchors 目录失败: %w", err)
		}
		dst := filepath.Join(anchorDir, caCertFileName)
		if err := os.WriteFile(dst, src, 0644); err != nil {
			return fmt.Errorf("复制 CA 证书到 %s 失败: %w", dst, err)
		}
		logf("CA 证书已复制到 %s", dst)

		logf("运行 trust extract-compat...")
		cmd := exec.Command(path, "extract-compat")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("trust extract-compat 失败: %w", err)
		}
		logf("CA 证书安装成功 (p11-kit/trust)")
		return nil
	}

	if path, err := exec.LookPath("update-ca-trust"); err == nil {
		anchorDir := "/etc/pki/ca-trust/source/anchors/"
		if err := os.MkdirAll(anchorDir, 0755); err != nil {
			return fmt.Errorf("创建 anchors 目录失败: %w", err)
		}
		dst := filepath.Join(anchorDir, caCertFileName)
		if err := os.WriteFile(dst, src, 0644); err != nil {
			return fmt.Errorf("复制 CA 证书到 %s 失败: %w", dst, err)
		}
		logf("CA 证书已复制到 %s", dst)

		logf("运行 update-ca-trust extract...")
		cmd := exec.Command(path, "extract")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("update-ca-trust extract 失败: %w", err)
		}
		logf("CA 证书安装成功 (ca-trust)")
		return nil
	}

	if path, err := exec.LookPath("update-ca-certificates"); err == nil {
		dstDir := "/usr/local/share/ca-certificates/"
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return fmt.Errorf("创建 CA 目录失败: %w", err)
		}
		dst := filepath.Join(dstDir, caCertFileName)
		if err := os.WriteFile(dst, src, 0644); err != nil {
			return fmt.Errorf("复制 CA 证书到 %s 失败: %w", dst, err)
		}
		logf("CA 证书已复制到 %s", dst)

		logf("运行 update-ca-certificates...")
		cmd := exec.Command(path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("update-ca-certificates 失败: %w", err)
		}
		logf("CA 证书安装成功 (Debian/Ubuntu)")
		return nil
	}

	return fmt.Errorf("未找到可用的 CA 证书管理工具 (trust/update-ca-trust/update-ca-certificates)，请手动安装 CA 证书")
}

func removeCALinux(logf func(string, ...interface{})) {
	candidates := []string{
		"/etc/ca-certificates/trust-source/anchors/" + caCertFileName,
		"/etc/pki/ca-trust/source/anchors/" + caCertFileName,
		"/usr/local/share/ca-certificates/" + caCertFileName,
	}

	removed := false
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if err := os.Remove(p); err != nil {
				logf("移除 %s 失败: %v", p, err)
			} else {
				logf("已移除 %s", p)
				removed = true
			}
		}
	}

	if !removed {
		logf("系统 CA 证书不存在，跳过移除")
		return
	}

	if path, err := exec.LookPath("trust"); err == nil {
		cmd := exec.Command(path, "extract-compat")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	} else if path, err := exec.LookPath("update-ca-trust"); err == nil {
		cmd := exec.Command(path, "extract")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	} else if path, err := exec.LookPath("update-ca-certificates"); err == nil {
		cmd := exec.Command(path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
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
