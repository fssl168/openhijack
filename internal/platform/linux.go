//go:build linux

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type LinuxPlatform struct{}

func init() {
	currentPlatform = &LinuxPlatform{}
}

func (p *LinuxPlatform) IsPrivileged() bool {
	return os.Geteuid() == 0
}

func (p *LinuxPlatform) GetHostsPath() string {
	return "/etc/hosts"
}

func (p *LinuxPlatform) GetConfigDir(homeDir string) string {
	return JoinPath(homeDir, ".config", "openhijack")
}

func (p *LinuxPlatform) GetDataDir(homeDir string) string {
	return JoinPath(homeDir, ".local", "share", "openhijack")
}

func (p *LinuxPlatform) Elevate(args []string, env []string) error {
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法获取可执行文件路径: %w", err)
	}

	sudoArgs := []string{"--preserve-env=OPENHIJACK_CONFIG", selfPath}
	sudoArgs = append(sudoArgs, args...)

	cmd := exec.Command("sudo", sudoArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo 执行失败: %w", err)
	}
	return nil
}

func (p *LinuxPlatform) ExecReplace(bin string, args []string, env []string) error {
	return syscall.Exec(bin, args, env)
}

func (p *LinuxPlatform) DetectDistro() string {
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range splitLines(string(data)) {
			if strings.HasPrefix(line, "ID=") {
				return trimQuote(strings.TrimPrefix(line, "ID="))
			}
		}
	}

	if _, err := os.Stat("/etc/debian_version"); err == nil {
		return "debian"
	}
	if _, err := os.Stat("/etc/redhat-release"); err == nil {
		return "rhel"
	}
	if _, err := os.Stat("/etc/arch-release"); err == nil {
		return "arch"
	}
	if _, err := os.Stat("/etc/alpine-release"); err == nil {
		return "alpine"
	}
	if _, err := os.Stat("/etc/suse-release"); err == nil {
		return "opensuse"
	}
	return "unknown"
}

func (p *LinuxPlatform) DetectCAMethod() string {
	methods := []struct {
		cmd  string
		name string
	}{
		{"update-ca-certificates", "debian"},
		{"update-ca-trust", "rhel"},
		{"trust", "arch"},
	}
	for _, m := range methods {
		if _, err := exec.LookPath(m.cmd); err == nil {
			return m.name
		}
	}
	return ""
}

func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

func trimQuote(s string) string {
	return strings.Trim(s, "\"'")
}
