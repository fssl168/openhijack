//go:build linux

package platform

import (
	"fmt"
	"os"
	"os/exec"
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
