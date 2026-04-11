package main

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveHomeDirPrefersSudoUser(t *testing.T) {
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("current user: %v", err)
	}

	home, err := resolveHomeDir(0, currentUser.Username, resolveUserHome, func() (string, error) {
		return "/root", nil
	})
	if err != nil {
		t.Fatalf("resolve home dir: %v", err)
	}
	if home != currentUser.HomeDir {
		t.Fatalf("home = %q, want %q", home, currentUser.HomeDir)
	}
}

func TestResolveHomeDirFallsBackToDefaultHome(t *testing.T) {
	home, err := resolveHomeDir(1000, "", resolveUserHome, func() (string, error) {
		return "/tmp/test-home", nil
	})
	if err != nil {
		t.Fatalf("resolve home dir: %v", err)
	}
	if home != "/tmp/test-home" {
		t.Fatalf("home = %q, want %q", home, "/tmp/test-home")
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
