//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

type WindowsPlatform struct{}

func init() {
	currentPlatform = &WindowsPlatform{}
}

func (p *WindowsPlatform) IsPrivileged() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.GetCurrentProcessToken()
	member, err := token.IsMember(sid)
	if err != nil {
		return false
	}

	return member
}

func (p *WindowsPlatform) GetHostsPath() string {
	return `C:\Windows\System32\drivers\etc\hosts`
}

func (p *WindowsPlatform) GetConfigDir(homeDir string) string {
	if appData := os.Getenv("APPDATA"); appData != "" {
		return JoinPath(appData, "openhijack")
	}
	return JoinPath(homeDir, ".config", "openhijack")
}

func (p *WindowsPlatform) GetDataDir(homeDir string) string {
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		return JoinPath(localAppData, "openhijack")
	}
	return JoinPath(homeDir, ".local", "share", "openhijack")
}

func (p *WindowsPlatform) Elevate(args []string, env []string) error {
	if p.IsPrivileged() {
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法获取可执行文件路径: %w", err)
	}

	params := strings.Join(args, " ")

	verb := "runas"
	cwd, _ := os.Getwd()
	showCmd := int32(windows.SW_NORMAL)

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exePath)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argPtr, _ := syscall.UTF16PtrFromString(params)

	err = windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, showCmd)
	if err != nil {
		uacErr := &UACElevationError{
			Err:      err,
			ExePath:  exePath,
			Params:   params,
			Fallback: "--http --port 8787",
		}
		uacErr.Hint = buildUACHint(uacErr)
		return uacErr
	}

	os.Exit(0)
	return nil
}

func (p *WindowsPlatform) DetectDistro() string {
	return "windows"
}

func (p *WindowsPlatform) DetectCAMethod() string {
	return "certutil"
}

func (p *WindowsPlatform) ExecReplace(bin string, args []string, env []string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec 替换失败（Windows 使用子进程）: %w", err)
	}
	os.Exit(0)
	return nil
}

type UACElevationError struct {
	Err      error
	ExePath  string
	Params   string
	Hint     string
	Fallback string
}

func (e *UACElevationError) Error() string {
	return fmt.Sprintf("UAC 权限提升失败: %v\n\n%s", e.Err, e.Hint)
}

func (e *UACElevationError) Unwrap() error {
	return e.Err
}

func buildUACHint(e *UACElevationError) string {
	hint := strings.Builder{}

	hint.WriteString("⚠️  需要管理员权限才能修改 hosts 文件和安装 CA 证书。\n\n")

	hint.WriteString("可能的原因：\n")
	hint.WriteString("  - 用户取消了 UAC 提示\n")
	hint.WriteString("  - UAC 已被系统管理员禁用\n")
	hint.WriteString("  - 账户没有管理员权限\n\n")

	hint.WriteString("解决方案：\n")
	hint.WriteString("  1. 右键点击此程序 → \"以管理员身份运行\"\n")
	hint.WriteString("  2. 使用管理员身份打开 PowerShell/CMD，然后重新执行此命令\n")
	fmt.Fprintf(&hint, "  3. 使用降级模式（无需管理员权限）:\n")
	fmt.Fprintf(&hint, "     openhijack serve %s\n", e.Fallback)

	return hint.String()
}
