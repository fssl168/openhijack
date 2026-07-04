package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadRejectsUnsupportedProvider(t *testing.T) {
	path := writeConfigFile(t, `
mapped_model_id = "mapped"

[[config_groups]]
name = "test"
provider = "made_up_unsupported"
api_url = "https://example.com"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected unsupported provider error")
	}
	if !strings.Contains(err.Error(), "不支持的 provider") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsUnknownProvider(t *testing.T) {
	path := writeConfigFile(t, `
mapped_model_id = "mapped"

[[config_groups]]
name = "test"
provider = "made_up"
api_url = "https://example.com"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected unknown provider error")
	}
	if !strings.Contains(err.Error(), "不支持的 provider") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadNormalizesOpenAIAlias(t *testing.T) {
	path := writeConfigFile(t, `
mapped_model_id = "mapped"

[[config_groups]]
name = "test"
provider = "openai"
api_url = "https://example.com"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got := cfg.CurrentGroup().Provider; got != ProviderOpenAIChatCompletion {
		t.Fatalf("provider = %q, want %q", got, ProviderOpenAIChatCompletion)
	}
}
