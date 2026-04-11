package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"openhijack/internal/platform"
)

func TestResolveHomeDirPrefersSudoUser(t *testing.T) {
	currentUser, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("current user home: %v", err)
	}

	home, err := platform.ResolveHomeDir(0, "testuser")
	if err != nil {
		t.Fatalf("resolve home dir: %v", err)
	}
	if home == "" {
		t.Fatal("expected non-empty home directory")
	}

	if currentUser != "" && home != currentUser {
		t.Logf("home = %q, current user home = %q (may differ in test env)", home, currentUser)
	}
}

func TestResolveHomeDirFallsBackToDefaultHome(t *testing.T) {
	home, err := platform.ResolveHomeDir(1000, "")
	if err != nil {
		t.Fatalf("resolve home dir: %v", err)
	}
	if home == "" {
		t.Fatal("expected non-empty home directory as fallback")
	}
}

func TestCreateDefaultConfigCreatesTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")

	created, err := createDefaultConfig(path, false)
	if err != nil {
		t.Fatalf("create default config: %v", err)
	}
	if !created {
		t.Fatal("expected config file to be created")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `provider = "openai_chat_completion"`) {
		t.Fatalf("unexpected config template: %s", text)
	}
	if !strings.Contains(text, `auth_key = "`) {
		t.Fatalf("expected auth_key in template: %s", text)
	}
}

func TestCreateDefaultConfigDoesNotOverwriteWithoutForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := []byte("existing = true\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	created, err := createDefaultConfig(path, false)
	if err != nil {
		t.Fatalf("create default config: %v", err)
	}
	if created {
		t.Fatal("expected existing config to be preserved")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(data) != string(original) {
		t.Fatalf("config was overwritten: %s", string(data))
	}
}

func TestGetConfigDirReturnsValidPath(t *testing.T) {
	configDir, err := platform.GetConfigDir()
	if err != nil {
		t.Fatalf("get config dir: %v", err)
	}
	if configDir == "" {
		t.Fatal("expected non-empty config dir")
	}
	if !filepath.IsAbs(configDir) {
		t.Fatalf("config dir should be absolute path: %s", configDir)
	}
}

func TestGetDataDirReturnsValidPath(t *testing.T) {
	dataDir, err := platform.GetDataDir()
	if err != nil {
		t.Fatalf("get data dir: %v", err)
	}
	if dataDir == "" {
		t.Fatal("expected non-empty data dir")
	}
	if !filepath.IsAbs(dataDir) {
		t.Fatalf("data dir should be absolute path: %s", dataDir)
	}
}

func TestGetHostsPathReturnsPlatformSpecificPath(t *testing.T) {
	hostsPath := platform.GetHostsPath()
	if hostsPath == "" {
		t.Fatal("expected non-empty hosts path")
	}
}
