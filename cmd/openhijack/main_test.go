package main

import (
	"os/user"
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
