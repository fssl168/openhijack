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
