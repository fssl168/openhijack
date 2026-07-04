package proxy

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeAuditConfig writes a minimal TOML config with an auth_key so handlers
// can be exercised end-to-end. Returns the config path.
func writeAuditConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestAuditLoggerWritesRecordsOnSuccess(t *testing.T) {
	cfgPath := writeAuditConfig(t, `
mapped_model_id = "mapped-model"
auth_key = "test-secret-key-1234567890"

[[config_groups]]
name = "test"
provider = "openai_chat_completion"
api_url = "https://example.com"
`)

	auditPath := filepath.Join(t.TempDir(), "audit.log")
	server, err := NewProxyServer(ServeOptions{
		ConfigPath:   cfgPath,
		AuditLogPath: auditPath,
	})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer test-secret-key-1234567890")
	rec := httptest.NewRecorder()
	server.setupRoutes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	// Audit log should now have one record.
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 audit line, got %d", len(lines))
	}

	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("parse audit entry: %v", err)
	}
	if entry["method"] != "GET" {
		t.Errorf("method = %v, want GET", entry["method"])
	}
	if entry["path"] != "/v1/models" {
		t.Errorf("path = %v, want /v1/models", entry["path"])
	}
	if entry["status"] != float64(http.StatusOK) {
		t.Errorf("status = %v, want 200", entry["status"])
	}
	if entry["model"] != "mapped-model" {
		t.Errorf("model = %v, want mapped-model", entry["model"])
	}
}

func TestAuditLoggerRecordsAuthFailure(t *testing.T) {
	cfgPath := writeAuditConfig(t, `
mapped_model_id = "mapped-model"
auth_key = "test-secret-key-1234567890"

[[config_groups]]
name = "test"
provider = "openai_chat_completion"
api_url = "https://example.com"
`)

	auditPath := filepath.Join(t.TempDir(), "audit.log")
	server, err := NewProxyServer(ServeOptions{
		ConfigPath:   cfgPath,
		AuditLogPath: auditPath,
	})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	server.setupRoutes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		t.Fatal("expected audit log to contain auth-failure record, got empty")
	}

	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &entry); err != nil {
		t.Fatalf("parse audit entry: %v", err)
	}
	if entry["status"] != float64(http.StatusUnauthorized) {
		t.Errorf("status = %v, want 401", entry["status"])
	}
	if entry["error"] != "auth failed" {
		t.Errorf("error = %v, want 'auth failed'", entry["error"])
	}
}

func TestAuditLoggerDisabledWhenPathEmpty(t *testing.T) {
	cfgPath := writeAuditConfig(t, `
mapped_model_id = "mapped-model"
auth_key = "test-secret-key-1234567890"

[[config_groups]]
name = "test"
provider = "openai_chat_completion"
api_url = "https://example.com"
`)

	server, err := NewProxyServer(ServeOptions{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	if server.audit != nil {
		t.Errorf("expected audit to be nil when AuditLogPath empty")
	}
	if server.auditFile != nil {
		t.Errorf("expected auditFile to be nil when AuditLogPath empty")
	}
}

func TestWatcherStatusReportsRunningWhenEnabled(t *testing.T) {
	cfgPath := writeAuditConfig(t, `
mapped_model_id = "mapped-model"
auth_key = "test-secret-key-1234567890"

[[config_groups]]
name = "test"
provider = "openai_chat_completion"
api_url = "https://example.com"
`)

	server, err := NewProxyServer(ServeOptions{
		ConfigPath:  cfgPath,
		WatchConfig: true,
	})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	status := server.GetWatcherStatus()
	if !status.Running {
		t.Errorf("expected watcher running=true, got false")
	}
}

func TestWatcherStatusDisabledWhenNotEnabled(t *testing.T) {
	cfgPath := writeAuditConfig(t, `
mapped_model_id = "mapped-model"
auth_key = "test-secret-key-1234567890"

[[config_groups]]
name = "test"
provider = "openai_chat_completion"
api_url = "https://example.com"
`)

	server, err := NewProxyServer(ServeOptions{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	status := server.GetWatcherStatus()
	if status.Running {
		t.Errorf("expected watcher running=false, got true")
	}
}

func TestReloadConfigManuallySwapsConfig(t *testing.T) {
	cfgPath := writeAuditConfig(t, `
mapped_model_id = "original-model"
auth_key = "test-secret-key-1234567890"

[[config_groups]]
name = "test"
provider = "openai_chat_completion"
api_url = "https://example.com"
`)

	server, err := NewProxyServer(ServeOptions{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	originalCfg := server.CurrentConfig()
	if originalCfg.MappedModelID != "original-model" {
		t.Fatalf("initial model = %v, want original-model", originalCfg.MappedModelID)
	}

	// Rewrite the config with a new mapped_model_id.
	newContent := strings.Replace(`
mapped_model_id = "updated-model"
auth_key = "test-secret-key-1234567890"

[[config_groups]]
name = "test"
provider = "openai_chat_completion"
api_url = "https://example.com"
`, "updated-model", "updated-model", 1)
	if err := os.WriteFile(cfgPath, []byte(newContent), 0600); err != nil {
		t.Fatalf("rewrite config: %v", err)
	}

	if err := server.ReloadConfigManually(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	updated := server.CurrentConfig()
	if updated.MappedModelID != "updated-model" {
		t.Errorf("after reload model = %v, want updated-model", updated.MappedModelID)
	}

	status := server.GetWatcherStatus()
	if status.LastReload == "" {
		t.Errorf("expected LastReload to be set after manual reload")
	}
}

func TestOnConfigReloadHandlesErrors(t *testing.T) {
	cfgPath := writeAuditConfig(t, `
mapped_model_id = "mapped-model"
auth_key = "test-secret-key-1234567890"

[[config_groups]]
name = "test"
provider = "openai_chat_completion"
api_url = "https://example.com"
`)

	server, err := NewProxyServer(ServeOptions{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	originalCfg := server.CurrentConfig()

	// Trigger reload with an error.
	server.onConfigReload(nil, errors.New("fake load error"))

	status := server.GetWatcherStatus()
	if status.LastError == "" {
		t.Errorf("expected LastError to be set on reload error")
	}

	// Config should NOT have been swapped.
	if server.CurrentConfig() != originalCfg {
		t.Errorf("config was swapped after error, expected unchanged")
	}
}

func TestCurrentConfigTransportAuthAreThreadSafe(t *testing.T) {
	cfgPath := writeAuditConfig(t, `
mapped_model_id = "mapped-model"
auth_key = "test-secret-key-1234567890"

[[config_groups]]
name = "test"
provider = "openai_chat_completion"
api_url = "https://example.com"
`)

	server, err := NewProxyServer(ServeOptions{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			_ = server.CurrentConfig()
			_ = server.CurrentTransport()
			_ = server.CurrentAuth()
		}
	}()

	// Concurrent reload attempts.
	for i := 0; i < 10; i++ {
		_ = server.ReloadConfigManually()
	}

	<-done
	// If we got here without a race detector failure, the test passes.
}

func TestClientIPFromRequest(t *testing.T) {
	tests := []struct {
		name     string
	xff      string
	xri      string
	remote   string
	expected string
	}{
		{"xff single", "1.2.3.4", "", "5.6.7.8:1234", "1.2.3.4"},
		{"xff multi", "1.2.3.4, 5.6.7.8", "", "9.9.9.9:1", "1.2.3.4"},
		{"xri fallback", "", "10.0.0.1", "9.9.9.9:1", "10.0.0.1"},
		{"remote fallback", "", "", "9.9.9.9:1", "9.9.9.9"},
		{"remote no port", "", "", "9.9.9.9", "9.9.9.9"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if tc.xri != "" {
				req.Header.Set("X-Real-IP", tc.xri)
			}
			req.RemoteAddr = tc.remote
			got := clientIPFromRequest(req)
			if got != tc.expected {
				t.Errorf("clientIPFromRequest = %q, want %q", got, tc.expected)
			}
		})
	}
}

// Ensure the watcher fires onConfigReload after a config file write.
func TestWatcherFiresOnConfigChange(t *testing.T) {
	cfgPath := writeAuditConfig(t, `
mapped_model_id = "original-model"
auth_key = "test-secret-key-1234567890"

[[config_groups]]
name = "test"
provider = "openai_chat_completion"
api_url = "https://example.com"
`)

	server, err := NewProxyServer(ServeOptions{
		ConfigPath:  cfgPath,
		WatchConfig: true,
	})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	// Rewrite config to trigger the watcher.
	newContent := `
mapped_model_id = "watcher-updated"
auth_key = "test-secret-key-1234567890"

[[config_groups]]
name = "test"
provider = "openai_chat_completion"
api_url = "https://example.com"
`
	if err := os.WriteFile(cfgPath, []byte(newContent), 0600); err != nil {
		t.Fatalf("rewrite config: %v", err)
	}

	// Wait up to 3 seconds for the watcher to fire (500ms debounce + load).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if server.CurrentConfig().MappedModelID == "watcher-updated" {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Errorf("watcher did not fire — model still %q", server.CurrentConfig().MappedModelID)
}

// TestOnConfigReloadedHook verifies that the OnConfigReloaded
// callback is invoked after a successful reload.
func TestOnConfigReloadedHook(t *testing.T) {
	cfgPath := writeAuditConfig(t, `
mapped_model_id = "original-model"
auth_key = "test-secret-key-1234567890"

[[config_groups]]
name = "test"
provider = "openai_chat_completion"
api_url = "https://example.com"
`)

	hookCh := make(chan WatcherStatus, 2)
	server, err := NewProxyServer(ServeOptions{
		ConfigPath: cfgPath,
		OnConfigReloaded: func(s WatcherStatus) {
			select {
			case hookCh <- s:
			default:
			}
		},
	})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	// Trigger a manual reload.
	if err := server.ReloadConfigManually(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	select {
	case status := <-hookCh:
		if status.LastReload == "" {
			t.Errorf("expected LastReload to be set in hook payload, got %+v", status)
		}
		if status.LastError != "" {
			t.Errorf("expected LastError empty on success, got %q", status.LastError)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("OnConfigReloaded hook was not invoked")
	}
}

// TestOnConfigReloadedHookOnError verifies that the hook fires with
// LastError set when reload fails.
func TestOnConfigReloadedHookOnError(t *testing.T) {
	cfgPath := writeAuditConfig(t, `
mapped_model_id = "original-model"
auth_key = "test-secret-key-1234567890"

[[config_groups]]
name = "test"
provider = "openai_chat_completion"
api_url = "https://example.com"
`)

	hookCh := make(chan WatcherStatus, 2)
	server, err := NewProxyServer(ServeOptions{
		ConfigPath: cfgPath,
		OnConfigReloaded: func(s WatcherStatus) {
			select {
			case hookCh <- s:
			default:
			}
		},
	})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	// Trigger reload with a synthetic error.
	server.onConfigReload(nil, errors.New("synthetic load failure"))

	select {
	case status := <-hookCh:
		if status.LastError == "" {
			t.Errorf("expected LastError to be set, got %+v", status)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("OnConfigReloaded hook was not invoked on error")
	}
}

// TestAuditLogPathAccessor verifies that the path is returned when
// audit logging is enabled and empty when disabled.
func TestAuditLogPathAccessor(t *testing.T) {
	cfgPath := writeAuditConfig(t, `
mapped_model_id = "m"
auth_key = "test-secret-key-1234567890"

[[config_groups]]
name = "test"
provider = "openai_chat_completion"
api_url = "https://example.com"
`)

	t.Run("enabled", func(t *testing.T) {
		auditPath := filepath.Join(t.TempDir(), "audit.log")
		server, err := NewProxyServer(ServeOptions{
			ConfigPath:   cfgPath,
			AuditLogPath: auditPath,
		})
		if err != nil {
			t.Fatalf("new proxy server: %v", err)
		}
		defer server.Stop()

		if got := server.AuditLogPath(); got != auditPath {
			t.Errorf("AuditLogPath() = %q, want %q", got, auditPath)
		}
		if server.AuditLogger() == nil {
			t.Errorf("AuditLogger() should not be nil when path is set")
		}
	})

	t.Run("disabled", func(t *testing.T) {
		server, err := NewProxyServer(ServeOptions{ConfigPath: cfgPath})
		if err != nil {
			t.Fatalf("new proxy server: %v", err)
		}
		defer server.Stop()

		if got := server.AuditLogPath(); got != "" {
			t.Errorf("AuditLogPath() = %q, want empty", got)
		}
		if server.AuditLogger() != nil {
			t.Errorf("AuditLogger() should be nil when no path is set")
		}
	})
}
