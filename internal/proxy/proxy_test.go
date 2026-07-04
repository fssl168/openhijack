package proxy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeProxyConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestSetupRoutesServesDefaultOpenAIModelsPath(t *testing.T) {
	path := writeProxyConfig(t, `
mapped_model_id = "mapped-model"
auth_key = "test-secret-key-1234567890"

[[config_groups]]
name = "test"
provider = "openai_chat_completion"
api_url = "https://example.com"
`)

	server, err := NewProxyServer(ServeOptions{ConfigPath: path})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer test-secret-key-1234567890")
	rec := httptest.NewRecorder()
	server.setupRoutes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "mapped-model") {
		t.Fatalf("expected mapped model in response: %s", rec.Body.String())
	}
}

func TestHandleOtherRequiresAuthentication(t *testing.T) {
	path := writeProxyConfig(t, `
mapped_model_id = "mapped-model"
auth_key = "secret"

[[config_groups]]
name = "test"
provider = "openai_chat_completion"
api_url = "https://example.com"
`)

	server, err := NewProxyServer(ServeOptions{ConfigPath: path})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(`{"input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.setupRoutes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHeadersConfigLoaded(t *testing.T) {
	path := writeProxyConfig(t, `
mapped_model_id = "mapped-model"

[[config_groups]]
name = "test-with-headers"
provider = "openai_chat_completion"
api_url = "https://example.com"
model_id = "gpt-4"

[config_groups.headers]
Authorization = "Bearer ak-Tpi51poca5pRPOBEP9jgOj"
Content-Type = "application/json"
X-Custom-Header = "custom-value"
`)

	server, err := NewProxyServer(ServeOptions{ConfigPath: path})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}

	group := server.config.CurrentGroup()
	if len(group.Headers) == 0 {
		t.Fatal("expected headers to be loaded, but got empty")
	}

	if got := group.Headers["Authorization"]; got != "Bearer ak-Tpi51poca5pRPOBEP9jgOj" {
		t.Fatalf("Authorization header = %q, want %q", got, "Bearer ak-Tpi51poca5pRPOBEP9jgOj")
	}
	if got := group.Headers["Content-Type"]; got != "application/json" {
		t.Fatalf("Content-Type header = %q, want %q", got, "application/json")
	}
	if got := group.Headers["X-Custom-Header"]; got != "custom-value" {
		t.Fatalf("X-Custom-Header = %q, want %q", got, "custom-value")
	}
}

func TestHeadersEmptyWhenNotConfigured(t *testing.T) {
	path := writeProxyConfig(t, `
mapped_model_id = "mapped-model"

[[config_groups]]
name = "test-no-headers"
provider = "openai_chat_completion"
api_url = "https://example.com"
`)

	server, err := NewProxyServer(ServeOptions{ConfigPath: path})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}

	group := server.config.CurrentGroup()
	if group.Headers == nil {
		t.Skip("headers map is nil when not configured (acceptable)")
	}
	if len(group.Headers) != 0 {
		t.Fatalf("expected no headers, got %d", len(group.Headers))
	}
}
