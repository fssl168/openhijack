//go:build darwin

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

type DarwinPlatform struct{}

func init() {
	currentPlatform = &DarwinPlatform{}
}

func (p *DarwinPlatform) IsPrivileged() bool {
	return os.Geteuid() == 0
}

func (p *DarwinPlatform) GetHostsPath() string {
	return "/etc/hosts"
}

func (p *DarwinPlatform) GetConfigDir(homeDir string) string {
	return JoinPath(homeDir, ".config", "openhijack")
}

func (p *DarwinPlatform) GetDataDir(homeDir string) string {
	return JoinPath(homeDir, ".local", "share", "openhijack")
}

func (p *DarwinPlatform) Elevate(args []string, env []string) error {
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

func (p *DarwinPlatform) ExecReplace(bin string, args []string, env []string) error {
	return syscall.Exec(bin, args, env)
}

func (p *DarwinPlatform) DetectDistro() string {
	return "macos"
}

func (p *DarwinPlatform) DetectCAMethod() string {
	return "security"
}
