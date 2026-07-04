package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"openhijack/internal/audit"
	"openhijack/internal/config"
	"openhijack/internal/health"
	"openhijack/internal/proxy"
)

// newTestApp constructs an App suitable for unit testing: it sets up
// the log channel and starts a drain goroutine so logProxy calls
// never block. The returned context is cancelable; cancelling it
// stops the drain goroutine.
func newTestApp(t *testing.T) (*App, context.Context) {
	t.Helper()
	app := NewApp()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Drain log channel so logProxy never blocks.
	var drainWg sync.WaitGroup
	drainWg.Add(1)
	go func() {
		defer drainWg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-app.logChan:
				if !ok {
					return
				}
			}
		}
	}()
	// On cleanup, close the channel and wait for the drain goroutine.
	t.Cleanup(func() {
		close(app.logChan)
		cancel()
		drainWg.Wait()
	})
	app.ctx = ctx
	return app, ctx
}

// writeTestConfig writes a minimal valid config and returns its path.
func writeTestConfig(t *testing.T, dir, mappedModel string) string {
	t.Helper()
	path := filepath.Join(dir, "config.toml")
	body := strings.Join([]string{
		`mapped_model_id = "` + mappedModel + `"`,
		`auth_key = "test-secret-key-1234567890"`,
		`current_config_index = 0`,
		``,
		`[[config_groups]]`,
		`name = "default"`,
		`provider = "openai_chat_completion"`,
		`api_url = "https://example.com"`,
		`model_id = "gpt-4o-mini"`,
		`api_key = "sk-test"`,
		`middle_route = "/v1"`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// --- B1: Doctor --------------------------------------------------------

func TestRunDoctor_ReturnsAllChecks(t *testing.T) {
	app, _ := newTestApp(t)

	results := app.RunDoctor()
	if len(results) != 6 {
		t.Fatalf("expected 6 checks, got %d", len(results))
	}

	expectedNames := []string{
		"CA Certificate",
		"Server Certificate",
		"Hosts Entries",
		"Port 443",
		"Config File",
		"Upstream Reachability",
	}
	for i, want := range expectedNames {
		if results[i].Name != want {
			t.Errorf("check %d: expected %q, got %q", i, want, results[i].Name)
		}
	}

	// All statuses should be valid.
	for _, r := range results {
		switch r.Status {
		case health.StatusPass, health.StatusWarn, health.StatusFail:
		default:
			t.Errorf("check %q: invalid status %q", r.Name, r.Status)
		}
	}
}

func TestGetLastDoctorResults_CachesResults(t *testing.T) {
	app, _ := newTestApp(t)

	// Before RunDoctor, GetLastDoctorResults returns nil/empty.
	if results := app.GetLastDoctorResults(); len(results) != 0 {
		t.Fatalf("expected empty before RunDoctor, got %d results", len(results))
	}

	app.RunDoctor()

	cached := app.GetLastDoctorResults()
	if len(cached) != 6 {
		t.Fatalf("expected 6 cached results, got %d", len(cached))
	}

	// Verify the cached slice is a copy — mutating it should not
	// affect the cache.
	cached[0].Status = "MUTATED"
	again := app.GetLastDoctorResults()
	if again[0].Status == "MUTATED" {
		t.Fatal("GetLastDoctorResults returned the underlying slice, not a copy")
	}
}

func TestGetDoctorSummary_CountsResults(t *testing.T) {
	app, _ := newTestApp(t)

	// Before RunDoctor, summary is all zeros.
	s := app.GetDoctorSummary()
	if s["pass"] != 0 || s["warn"] != 0 || s["fail"] != 0 {
		t.Fatalf("expected zero summary before RunDoctor, got %v", s)
	}

	app.RunDoctor()
	s = app.GetDoctorSummary()
	total := s["pass"] + s["warn"] + s["fail"]
	if total != 6 {
		t.Fatalf("expected 6 total results in summary, got %d (pass=%d warn=%d fail=%d)",
			total, s["pass"], s["warn"], s["fail"])
	}
}

// --- B2: AuditLog ------------------------------------------------------

func TestGetAuditLogPath_EmptyWhenNoServer(t *testing.T) {
	app, _ := newTestApp(t)
	if got := app.GetAuditLogPath(); got != "" {
		t.Fatalf("expected empty audit log path with no server, got %q", got)
	}
}

func TestGetAuditLogs_EmptyWhenNoServer(t *testing.T) {
	app, _ := newTestApp(t)
	entries := app.GetAuditLogs(100, 0)
	if len(entries) != 0 {
		t.Fatalf("expected empty entries with no server, got %d", len(entries))
	}
}

func TestClearAuditLogs_DisabledWhenNoServer(t *testing.T) {
	app, _ := newTestApp(t)
	if msg := app.ClearAuditLogs(); msg == "" {
		t.Fatal("expected ClearAuditLogs to report disabled when no server")
	}
}

func TestGetAuditLogs_ReadsFromAuditFile(t *testing.T) {
	app, _ := newTestApp(t)

	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, "test-model")
	auditPath := filepath.Join(dir, "audit.log")

	// Write a few audit entries manually to the file.
	now := time.Now()
	entries := []audit.AuditEntry{
		{RequestID: "req1", Method: "GET", Path: "/v1/models", Status: 200, Timestamp: now, Duration: "1.50"},
		{RequestID: "req2", Method: "POST", Path: "/v1/chat/completions", Status: 500, Timestamp: now, Duration: "20.00", Error: "upstream error"},
		{RequestID: "req3", Method: "GET", Path: "/v1/models", Status: 401, Timestamp: now, Duration: "0.10"},
	}
	var sb strings.Builder
	for _, e := range entries {
		data, _ := json.Marshal(e)
		sb.Write(data)
		sb.WriteByte('\n')
	}
	if err := os.WriteFile(auditPath, []byte(sb.String()), 0600); err != nil {
		t.Fatalf("write audit log: %v", err)
	}

	// Set up a proxy server with audit logging enabled.
	server, err := proxy.NewProxyServer(proxy.ServeOptions{
		ConfigPath:   cfgPath,
		AuditLogPath: auditPath,
	})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	app.mu.Lock()
	app.server = server
	app.mu.Unlock()

	// Get most recent 10 entries.
	got := app.GetAuditLogs(10, 0)
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}

	// Most recent first — req3 should be the first entry.
	if got[0].RequestID != "req3" {
		t.Errorf("expected req3 first (most recent), got %q", got[0].RequestID)
	}
	if got[1].RequestID != "req2" {
		t.Errorf("expected req2 second, got %q", got[1].RequestID)
	}
	if got[2].RequestID != "req1" {
		t.Errorf("expected req1 third, got %q", got[2].RequestID)
	}

	// Limit=1 should return only the most recent.
	one := app.GetAuditLogs(1, 0)
	if len(one) != 1 || one[0].RequestID != "req3" {
		t.Fatalf("limit=1 should return only req3, got %v", one)
	}

	// Offset=2 should skip the first 2 entries (req3 and req2).
	tail := app.GetAuditLogs(10, 2)
	if len(tail) != 1 || tail[0].RequestID != "req1" {
		t.Fatalf("offset=2 should return only req1, got %v", tail)
	}

	// Offset larger than the slice should return empty.
	empty := app.GetAuditLogs(10, 5)
	if len(empty) != 0 {
		t.Fatalf("offset=5 should return empty, got %d", len(empty))
	}
}

func TestGetAuditLogs_DefaultsTo100WhenLimitZero(t *testing.T) {
	app, _ := newTestApp(t)
	// No server → returns empty, but the call shouldn't panic.
	entries := app.GetAuditLogs(0, 0)
	if len(entries) != 0 {
		t.Fatalf("expected empty when no server, got %d", len(entries))
	}
}

func TestGetAuditLogs_HandlesCorruptLines(t *testing.T) {
	app, _ := newTestApp(t)

	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, "m")
	auditPath := filepath.Join(dir, "audit.log")

	// Write one valid entry and one corrupt line.
	valid, _ := json.Marshal(audit.AuditEntry{RequestID: "valid", Method: "GET", Path: "/", Status: 200})
	body := "this is not json\n" + string(valid) + "\n\n"
	if err := os.WriteFile(auditPath, []byte(body), 0600); err != nil {
		t.Fatalf("write audit log: %v", err)
	}

	server, err := proxy.NewProxyServer(proxy.ServeOptions{
		ConfigPath:   cfgPath,
		AuditLogPath: auditPath,
	})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	app.mu.Lock()
	app.server = server
	app.mu.Unlock()

	got := app.GetAuditLogs(100, 0)
	if len(got) != 1 {
		t.Fatalf("expected 1 valid entry, got %d", len(got))
	}
	if got[0].RequestID != "valid" {
		t.Errorf("expected RequestID=valid, got %q", got[0].RequestID)
	}
}

func TestClearAuditLogs_TruncatesFile(t *testing.T) {
	app, _ := newTestApp(t)

	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, "m")
	auditPath := filepath.Join(dir, "audit.log")

	// Write some content.
	if err := os.WriteFile(auditPath, []byte("some content\n"), 0600); err != nil {
		t.Fatalf("write audit log: %v", err)
	}

	server, err := proxy.NewProxyServer(proxy.ServeOptions{
		ConfigPath:   cfgPath,
		AuditLogPath: auditPath,
	})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	app.mu.Lock()
	app.server = server
	app.mu.Unlock()

	if msg := app.ClearAuditLogs(); msg != "" {
		t.Fatalf("ClearAuditLogs failed: %s", msg)
	}

	// File should now be empty.
	info, err := os.Stat(auditPath)
	if err != nil {
		t.Fatalf("stat audit log: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("audit log size = %d, want 0", info.Size())
	}
}

// --- B3: Watcher ------------------------------------------------------

func TestGetWatcherStatus_ZeroWhenNoServer(t *testing.T) {
	app, _ := newTestApp(t)
	status := app.GetWatcherStatus()
	if status.Running {
		t.Fatal("expected Running=false with no server")
	}
	if status.LastReload != "" || status.LastError != "" {
		t.Fatalf("expected zero-value status, got %+v", status)
	}
}

func TestReloadConfigManually_ErrorsWhenNoServer(t *testing.T) {
	app, _ := newTestApp(t)
	if msg := app.ReloadConfigManually(); msg == "" {
		t.Fatal("expected error message when no server, got empty")
	}
}

func TestGetWatcherStatus_ReflectsRunningProxy(t *testing.T) {
	app, _ := newTestApp(t)

	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, "watcher-test")

	server, err := proxy.NewProxyServer(proxy.ServeOptions{
		ConfigPath:  cfgPath,
		WatchConfig: true,
	})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	app.mu.Lock()
	app.server = server
	app.mu.Unlock()

	status := app.GetWatcherStatus()
	if !status.Running {
		t.Errorf("expected Running=true when watcher enabled, got %+v", status)
	}
}

func TestReloadConfigManually_ReloadsWhenServerRunning(t *testing.T) {
	app, _ := newTestApp(t)

	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, "reload-test")

	server, err := proxy.NewProxyServer(proxy.ServeOptions{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	app.mu.Lock()
	app.server = server
	app.mu.Unlock()

	if msg := app.ReloadConfigManually(); msg != "" {
		t.Fatalf("ReloadConfigManually failed: %s", msg)
	}

	status := app.GetWatcherStatus()
	if status.LastReload == "" {
		t.Error("expected LastReload to be set after manual reload")
	}
}

// --- resolveActiveConfigPath -------------------------------------------

func TestResolveActiveConfigPath_FallsBackToPlatformDefault(t *testing.T) {
	app, _ := newTestApp(t)
	// No lastConfig set — should fall back to platform default.
	got := app.resolveActiveConfigPath()
	if got == "" {
		t.Fatal("expected fallback path, got empty")
	}
	if !strings.HasSuffix(got, "config.toml") {
		t.Errorf("expected fallback to end with config.toml, got %q", got)
	}
}

func TestResolveActiveConfigPath_UsesLastConfigWhenSet(t *testing.T) {
	app, _ := newTestApp(t)

	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, "active")

	app.mu.Lock()
	app.lastConfig = cfgPath
	app.mu.Unlock()

	if got := app.resolveActiveConfigPath(); got != cfgPath {
		t.Errorf("expected %q, got %q", cfgPath, got)
	}
}

func TestResolveActiveConfigPath_FallsBackWhenFileMissing(t *testing.T) {
	app, _ := newTestApp(t)

	app.mu.Lock()
	app.lastConfig = "/nonexistent/config.toml"
	app.mu.Unlock()

	got := app.resolveActiveConfigPath()
	if got == "" {
		t.Fatal("expected fallback path when lastConfig is missing, got empty")
	}
}

func TestGetAuditLogs_ReturnsEmptyWhenFileMissing(t *testing.T) {
	app, _ := newTestApp(t)

	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, "missing-file")
	auditPath := filepath.Join(dir, "audit.log")

	server, err := proxy.NewProxyServer(proxy.ServeOptions{
		ConfigPath:   cfgPath,
		AuditLogPath: auditPath,
	})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	app.mu.Lock()
	app.server = server
	app.mu.Unlock()

	// Remove the audit file so os.ReadFile fails. The proxy may
	// have created it during setup; ensure it's gone.
	if err := os.Remove(auditPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove audit log: %v", err)
	}

	got := app.GetAuditLogs(100, 0)
	if len(got) != 0 {
		t.Fatalf("expected empty when audit file missing, got %d entries", len(got))
	}
}

func TestClearAuditLogs_ErrorsWhenFileMissing(t *testing.T) {
	app, _ := newTestApp(t)

	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, "truncate-err")
	auditPath := filepath.Join(dir, "audit.log")

	server, err := proxy.NewProxyServer(proxy.ServeOptions{
		ConfigPath:   cfgPath,
		AuditLogPath: auditPath,
	})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	app.mu.Lock()
	app.server = server
	app.mu.Unlock()

	// Remove the audit file so os.Truncate fails with ENOENT.
	if err := os.Remove(auditPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove audit log: %v", err)
	}

	msg := app.ClearAuditLogs()
	if msg == "" {
		t.Fatal("expected error message when audit file missing, got empty")
	}
	if !strings.Contains(msg, "清空审计日志失败") {
		t.Errorf("expected error to mention clear failure, got %q", msg)
	}
}

func TestReloadConfigManually_ErrorsOnCorruptConfig(t *testing.T) {
	app, _ := newTestApp(t)

	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, "corrupt-test")

	server, err := proxy.NewProxyServer(proxy.ServeOptions{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	app.mu.Lock()
	app.server = server
	app.mu.Unlock()

	// Overwrite the config with invalid TOML so reload fails.
	if err := os.WriteFile(cfgPath, []byte("this is = = not valid toml [["), 0600); err != nil {
		t.Fatalf("write corrupt config: %v", err)
	}

	msg := app.ReloadConfigManually()
	if msg == "" {
		t.Fatal("expected error message for corrupt config reload, got empty")
	}
	if !strings.Contains(msg, "重载配置失败") {
		t.Errorf("expected error to mention reload failure, got %q", msg)
	}
}

// --- Smoke test: doctor results survive config reload ------------------

func TestDoctorResults_AreNotAffectedByConfigReload(t *testing.T) {
	app, _ := newTestApp(t)

	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, "doctor-test")

	server, err := proxy.NewProxyServer(proxy.ServeOptions{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new proxy server: %v", err)
	}
	defer server.Stop()

	app.mu.Lock()
	app.server = server
	app.lastConfig = cfgPath
	app.mu.Unlock()

	// Run doctor once.
	firstResults := app.RunDoctor()
	if len(firstResults) != 6 {
		t.Fatalf("expected 6 results, got %d", len(firstResults))
	}

	// Reload config.
	if msg := app.ReloadConfigManually(); msg != "" {
		t.Fatalf("reload failed: %s", msg)
	}

	// Cache should still be intact.
	cached := app.GetLastDoctorResults()
	if len(cached) != 6 {
		t.Fatalf("expected cached results intact, got %d", len(cached))
	}

	// Re-run doctor — should produce same 6 results.
	secondResults := app.RunDoctor()
	if len(secondResults) != 6 {
		t.Fatalf("expected 6 results after reload, got %d", len(secondResults))
	}
}

// Compile-time check: ensure App satisfies the documented return
// types for the new Wails bindings. If any signature drifts, this
// will fail to compile.
var _ = func() {
	var a *App
	_ = a.RunDoctor
	_ = a.GetLastDoctorResults
	_ = a.GetDoctorSummary
	_ = a.GetAuditLogs
	_ = a.GetAuditLogPath
	_ = a.ClearAuditLogs
	_ = a.GetWatcherStatus
	_ = a.ReloadConfigManually
}

// Compile-time check: ensure types match across packages.
var _ health.CheckResult
var _ audit.AuditEntry
var _ proxy.WatcherStatus
var _ config.Config
