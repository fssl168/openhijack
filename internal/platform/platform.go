package platform

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

type Platform interface {
	IsPrivileged() bool
	GetHostsPath() string
	GetConfigDir(homeDir string) string
	GetDataDir(homeDir string) string
	Elevate(args []string, env []string) error
	ExecReplace(bin string, args []string, env []string) error
}

var currentPlatform Platform

func Current() Platform {
	if currentPlatform == nil {
		return &defaultPlatform{}
	}
	return currentPlatform
}

type defaultPlatform struct{}

func (p *defaultPlatform) IsPrivileged() bool {
	return false
}

func (p *defaultPlatform) GetHostsPath() string {
	return "/etc/hosts"
}

func (p *defaultPlatform) GetConfigDir(homeDir string) string {
	return JoinPath(homeDir, ".config", "openhijack")
}

func (p *defaultPlatform) GetDataDir(homeDir string) string {
	return JoinPath(homeDir, ".local", "share", "openhijack")
}

func (p *defaultPlatform) Elevate(args []string, env []string) error {
	return fmt.Errorf("elevate not implemented for this platform")
}

func (p *defaultPlatform) ExecReplace(bin string, args []string, env []string) error {
	return fmt.Errorf("exec replace not implemented for this platform")
}

func IsPrivileged() bool {
	return Current().IsPrivileged()
}

func GetHostsPath() string {
	return Current().GetHostsPath()
}

func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return Current().GetConfigDir(home), nil
}

func GetDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return Current().GetDataDir(home), nil
}

func Elevate(args []string, env []string) error {
	return Current().Elevate(args, env)
}

func ExecReplace(bin string, args []string, env []string) error {
	return Current().ExecReplace(bin, args, env)
}

func ResolveHomeDir(euid int, sudoUser string) (string, error) {
	if euid == 0 && sudoUser != "" {
		if u, err := user.Lookup(sudoUser); err == nil {
			return u.HomeDir, nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if home == "" {
		return "", os.ErrNotExist
	}
	return home, nil
}

func EnsureDir(path string, perm os.FileMode) error {
	if err := os.MkdirAll(path, perm); err != nil {
		return err
	}
	return nil
}

func JoinPath(elem ...string) string {
	return filepath.Join(elem...)
}
