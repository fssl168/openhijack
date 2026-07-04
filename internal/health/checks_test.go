package health

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"openhijack/internal/hosts"
	"openhijack/internal/platform"
)

// --- Helpers -------------------------------------------------------------

// newTempDir creates a fresh data dir for the test and returns its
// path plus a cleanup function.
func newTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "openhijack-health-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// writeConfig writes a minimal TOML config file to dir/config.toml
// and returns its path.
func writeConfig(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

const validConfigTOML = `mapped_model_id = "test-model"
auth_key = "test-secret-key-1234567890"
current_config_index = 0

[[config_groups]]
name = "default"
provider = "openai_chat_completion"
api_url = "%s"
model_id = "gpt-4o-mini"
api_key = "sk-test"
middle_route = "/v1"
`

// --- Tests ---------------------------------------------------------------

func TestStatusConstants(t *testing.T) {
	if StatusPass != "PASS" || StatusWarn != "WARN" || StatusFail != "FAIL" {
		t.Fatalf("status constants changed; consumers depend on stable strings")
	}
}

func TestCheckCACertificate_Missing(t *testing.T) {
	dir := newTempDir(t)
	r := CheckCACertificate(dir)
	if r.Status != StatusFail {
		t.Fatalf("expected FAIL for missing CA, got %s (%s)", r.Status, r.Detail)
	}
	if r.FixHint == "" {
		t.Fatalf("FAIL result should include a FixHint")
	}
	if r.Name != "CA Certificate" {
		t.Fatalf("unexpected name %q", r.Name)
	}
}

func TestCheckServerCert_Missing(t *testing.T) {
	dir := newTempDir(t)
	r := CheckServerCert(dir)
	if r.Status != StatusFail {
		t.Fatalf("expected FAIL for missing server cert, got %s", r.Status)
	}
	if !strings.Contains(r.FixHint, "elevate") {
		t.Fatalf("FixHint should mention elevate: %q", r.FixHint)
	}
}

func TestCheckPortAvailability_Free(t *testing.T) {
	// Pick an ephemeral port by listening once and closing it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	_ = ln.Close()

	r := CheckPortAvailability(addr.Port)
	if r.Status != StatusPass {
		t.Fatalf("expected PASS on free port %d, got %s (%s)", addr.Port, r.Status, r.Detail)
	}
}

func TestCheckPortAvailability_InUse(t *testing.T) {
	// Hold a port open and ensure CheckPortAvailability reports WARN.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)
	r := CheckPortAvailability(addr.Port)
	if r.Status != StatusWarn {
		t.Fatalf("expected WARN on in-use port %d, got %s (%s)", addr.Port, r.Status, r.Detail)
	}
}

func TestCheckPortAvailability_Privileged(t *testing.T) {
	// Port 1 requires root on POSIX; if we happen to be root this test
	// is skipped rather than asserting FAIL (root can bind port 1).
	if os.Geteuid() == 0 {
		t.Skip("running as root; cannot test permission failure")
	}
	r := CheckPortAvailability(1)
	if r.Status != StatusFail {
		t.Fatalf("expected FAIL on privileged port 1, got %s (%s)", r.Status, r.Detail)
	}
	if !strings.Contains(r.FixHint, "sudo") && !strings.Contains(r.FixHint, "8443") {
		t.Fatalf("FixHint should mention sudo or 8443: %q", r.FixHint)
	}
}

func TestCheckConfigFile_Valid(t *testing.T) {
	dir := newTempDir(t)
	path := writeConfig(t, dir, fmt.Sprintf(validConfigTOML, "https://api.openai.com"))

	r := CheckConfigFile(path)
	if r.Status != StatusPass {
		t.Fatalf("expected PASS for valid config, got %s (%s)", r.Status, r.Detail)
	}
	if !strings.Contains(r.Detail, "provider=") || !strings.Contains(r.Detail, "model=") {
		t.Fatalf("Detail should mention provider/model: %q", r.Detail)
	}
}

func TestCheckConfigFile_Missing(t *testing.T) {
	r := CheckConfigFile("/nonexistent/openhijack-health/config.toml")
	if r.Status != StatusFail {
		t.Fatalf("expected FAIL for missing config, got %s", r.Status)
	}
	if !strings.Contains(r.FixHint, "init") {
		t.Fatalf("FixHint should mention init: %q", r.FixHint)
	}
}

func TestCheckUpstreamReachability_Reachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dir := newTempDir(t)
	path := writeConfig(t, dir, fmt.Sprintf(validConfigTOML, server.URL))

	r := CheckUpstreamReachability(path, 2*time.Second)
	if r.Status != StatusPass {
		t.Fatalf("expected PASS for reachable upstream, got %s (%s)", r.Status, r.Detail)
	}
	if !strings.Contains(r.Detail, "HTTP 200") {
		t.Fatalf("Detail should mention HTTP 200: %q", r.Detail)
	}
}

func TestCheckUpstreamReachability_Unreachable(t *testing.T) {
	// Use a non-routable address to guarantee failure.
	dir := newTempDir(t)
	path := writeConfig(t, dir, fmt.Sprintf(validConfigTOML, "http://192.0.2.1:1/"))

	r := CheckUpstreamReachability(path, 200*time.Millisecond)
	if r.Status != StatusWarn {
		t.Fatalf("expected WARN for unreachable upstream, got %s (%s)", r.Status, r.Detail)
	}
	if !strings.Contains(r.Detail, "unreachable") && !strings.Contains(r.Detail, "timeout") {
		t.Fatalf("Detail should mention unreachability: %q", r.Detail)
	}
}

func TestCheckUpstreamReachability_EmptyURL(t *testing.T) {
	dir := newTempDir(t)
	body := `mapped_model_id = "x"
auth_key = "test-secret-key-1234567890"
current_config_index = 0

[[config_groups]]
name = "default"
provider = "openai_chat_completion"
api_url = ""
model_id = "gpt-4o-mini"
api_key = "sk-test"
middle_route = "/v1"
`
	path := writeConfig(t, dir, body)

	r := CheckUpstreamReachability(path, 1*time.Second)
	if r.Status != StatusWarn {
		t.Fatalf("expected WARN for empty api_url, got %s (%s)", r.Status, r.Detail)
	}
}

func TestCheckUpstreamReachability_BadConfig(t *testing.T) {
	// Path that doesn't exist → config.Load fails → check should skip.
	r := CheckUpstreamReachability("/nonexistent/config.toml", 1*time.Second)
	if r.Status != StatusPass {
		t.Fatalf("expected PASS (skipped) on bad config, got %s (%s)", r.Status, r.Detail)
	}
	if !strings.Contains(r.Detail, "Skipped") {
		t.Fatalf("Detail should mention skip: %q", r.Detail)
	}
}

func TestCheckHostsEntries_NoFile(t *testing.T) {
	// On a typical CI box the hosts file is readable but if we point
	// at a missing path via the platform helper we can exercise the
	// error branch indirectly. Since platform.GetHostsPath() returns
	// the system path, we instead verify behavior by checking that
	// the function returns one of the documented statuses.
	r := CheckHostsEntries()
	switch r.Status {
	case StatusPass, StatusWarn, StatusFail:
		// all valid outcomes depending on environment
	default:
		t.Fatalf("unexpected status %q for hosts check", r.Status)
	}
	if r.Name != "Hosts Entries" {
		t.Fatalf("unexpected name %q", r.Name)
	}
}

func TestSummary_Counts(t *testing.T) {
	results := []CheckResult{
		{Status: StatusPass},
		{Status: StatusPass},
		{Status: StatusWarn},
		{Status: StatusFail},
		{Status: StatusFail},
		{Status: StatusFail},
	}
	pass, warn, fail := Summary(results)
	if pass != 2 || warn != 1 || fail != 3 {
		t.Fatalf("expected 2/1/3, got %d/%d/%d", pass, warn, fail)
	}
}

func TestRunAllChecks_OrderAndCount(t *testing.T) {
	dir := newTempDir(t)
	path := writeConfig(t, dir, fmt.Sprintf(validConfigTOML, "https://api.openai.com"))

	results := RunAllChecks(Options{
		DataDir:        dir,
		ConfigPath:     path,
		Port:           0,             // exercise default
		UpstreamTimeout: 500 * time.Millisecond,
	})

	if len(results) != 6 {
		t.Fatalf("expected 6 checks, got %d", len(results))
	}

	expectedOrder := []string{
		"CA Certificate",
		"Server Certificate",
		"Hosts Entries",
		"Port 443",
		"Config File",
		"Upstream Reachability",
	}
	for i, want := range expectedOrder {
		if results[i].Name != want {
			t.Fatalf("check %d: expected %q, got %q", i, want, results[i].Name)
		}
	}

	// CA cert and Server cert are missing in the temp dir → FAIL.
	if results[0].Status != StatusFail {
		t.Fatalf("CA cert should be FAIL in empty dir, got %s", results[0].Status)
	}
	if results[1].Status != StatusFail {
		t.Fatalf("Server cert should be FAIL in empty dir, got %s", results[1].Status)
	}

	// Config should be PASS — we wrote a valid file.
	if results[4].Status != StatusPass {
		t.Fatalf("Config check should be PASS, got %s (%s)", results[4].Status, results[4].Detail)
	}
}

func TestRunAllChecks_DefaultPort(t *testing.T) {
	dir := newTempDir(t)
	path := writeConfig(t, dir, fmt.Sprintf(validConfigTOML, "https://api.openai.com"))

	results := RunAllChecks(Options{DataDir: dir, ConfigPath: path})
	// Port check at default (443) — status depends on environment but
	// the Name must reflect the default port.
	portCheck := results[3]
	if portCheck.Name != "Port 443" {
		t.Fatalf("expected default port 443, got %q", portCheck.Name)
	}
}

// --- CA / ServerCert "found" paths --------------------------------------

// seedCAFiles writes minimal placeholder files so HasCA() returns true.
// We do not generate real certs — the health check only checks file
// existence via os.Stat.
func seedCAFiles(t *testing.T, dataDir string) {
	t.Helper()
	caDir := filepath.Join(dataDir, "ca")
	if err := os.MkdirAll(caDir, 0755); err != nil {
		t.Fatalf("mkdir ca dir: %v", err)
	}
	for _, name := range []string{"ca.crt", "ca.key"} {
		if err := os.WriteFile(filepath.Join(caDir, name), []byte("placeholder"), 0600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

// seedServerCertFiles writes the api.openai.com cert pair so
// HasServerCert() returns true.
func seedServerCertFiles(t *testing.T, dataDir string) {
	t.Helper()
	caDir := filepath.Join(dataDir, "ca")
	if err := os.MkdirAll(caDir, 0755); err != nil {
		t.Fatalf("mkdir ca dir: %v", err)
	}
	for _, name := range []string{"api.openai.com.crt", "api.openai.com.key"} {
		if err := os.WriteFile(filepath.Join(caDir, name), []byte("placeholder"), 0600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func TestCheckCACertificate_Found(t *testing.T) {
	dir := newTempDir(t)
	seedCAFiles(t, dir)
	r := CheckCACertificate(dir)
	if r.Status != StatusPass {
		t.Fatalf("expected PASS for present CA, got %s (%s)", r.Status, r.Detail)
	}
	if !strings.Contains(r.Detail, "Found") {
		t.Fatalf("Detail should mention Found: %q", r.Detail)
	}
}

func TestCheckServerCert_Found(t *testing.T) {
	dir := newTempDir(t)
	seedServerCertFiles(t, dir)
	r := CheckServerCert(dir)
	if r.Status != StatusPass {
		t.Fatalf("expected PASS for present server cert, got %s (%s)", r.Status, r.Detail)
	}
}

// --- CheckHostsEntriesAt (path-parameterized variant) -------------------

func writeHostsFile(t *testing.T, body string) string {
	t.Helper()
	dir := newTempDir(t)
	path := filepath.Join(dir, "hosts")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("write hosts: %v", err)
	}
	return path
}

func TestCheckHostsEntriesAt_NoFile(t *testing.T) {
	r := CheckHostsEntriesAt(filepath.Join(newTempDir(t), "missing-hosts"))
	if r.Status != StatusFail {
		t.Fatalf("expected FAIL for missing hosts file, got %s", r.Status)
	}
	if !strings.Contains(r.Detail, "Cannot read") {
		t.Fatalf("Detail should mention Cannot read: %q", r.Detail)
	}
}

func TestCheckHostsEntriesAt_NoMarker(t *testing.T) {
	path := writeHostsFile(t, "127.0.0.1 localhost\n")
	r := CheckHostsEntriesAt(path)
	if r.Status != StatusFail {
		t.Fatalf("expected FAIL when no marker present, got %s", r.Status)
	}
	if !strings.Contains(r.Detail, "No OpenHijack entries") {
		t.Fatalf("Detail should mention missing entries: %q", r.Detail)
	}
}

func TestCheckHostsEntriesAt_AllDomains(t *testing.T) {
	domains := hosts.Domains()
	marker := hosts.Markers()
	var sb strings.Builder
	sb.WriteString(marker)
	sb.WriteString("\n")
	for _, d := range domains {
		sb.WriteString("127.0.0.1 ")
		sb.WriteString(d)
		sb.WriteString("\n")
	}
	path := writeHostsFile(t, sb.String())

	r := CheckHostsEntriesAt(path)
	if r.Status != StatusPass {
		t.Fatalf("expected PASS for all domains, got %s (%s)", r.Status, r.Detail)
	}
	if !strings.Contains(r.Detail, "All domains mapped") {
		t.Fatalf("Detail should mention All domains mapped: %q", r.Detail)
	}
}

func TestCheckHostsEntriesAt_PartialDomains(t *testing.T) {
	domains := hosts.Domains()
	if len(domains) < 2 {
		t.Skip("test requires at least 2 known domains")
	}
	marker := hosts.Markers()
	var sb strings.Builder
	sb.WriteString(marker)
	sb.WriteString("\n")
	// Only include the first domain.
	sb.WriteString("127.0.0.1 ")
	sb.WriteString(domains[0])
	sb.WriteString("\n")
	path := writeHostsFile(t, sb.String())

	r := CheckHostsEntriesAt(path)
	if r.Status != StatusWarn {
		t.Fatalf("expected WARN for partial mapping, got %s (%s)", r.Status, r.Detail)
	}
	if !strings.Contains(r.Detail, "Partial") {
		t.Fatalf("Detail should mention Partial: %q", r.Detail)
	}
}

// --- Upstream reachability edge cases -----------------------------------

func TestCheckUpstreamReachability_InvalidURL(t *testing.T) {
	dir := newTempDir(t)
	body := `mapped_model_id = "x"
auth_key = "test-secret-key-1234567890"
current_config_index = 0

[[config_groups]]
name = "default"
provider = "openai_chat_completion"
api_url = "http://[::1:invalid"
model_id = "gpt-4o-mini"
api_key = "sk-test"
middle_route = "/v1"
`
	path := writeConfig(t, dir, body)
	r := CheckUpstreamReachability(path, 1*time.Second)
	if r.Status != StatusFail {
		t.Fatalf("expected FAIL for invalid URL, got %s (%s)", r.Status, r.Detail)
	}
	if !strings.Contains(r.Detail, "Invalid api_url") {
		t.Fatalf("Detail should mention Invalid api_url: %q", r.Detail)
	}
	if !strings.Contains(r.FixHint, "api_url") {
		t.Fatalf("FixHint should mention api_url: %q", r.FixHint)
	}
}

// --- isPermissionError: nil error --------------------------------------

func TestIsPermissionError_Nil(t *testing.T) {
	if isPermissionError(nil) {
		t.Fatal("nil error should not be a permission error")
	}
}

// --- Cross-checks against existing helpers ------------------------------

func TestCheckHostsEntries_UsesHostsPackage(t *testing.T) {
	// Sanity: ensure the markers/domains we look for are still the
	// ones exported by the hosts package. This is a compile-time
	// guarantee plus a basic non-empty assertion.
	if hosts.Markers() == "" {
		t.Fatal("hosts.Markers() returned empty string")
	}
	if len(hosts.Domains()) == 0 {
		t.Fatal("hosts.Domains() returned empty slice")
	}
}

func TestCheckConfigFile_UsesPlatformConfig(t *testing.T) {
	// Sanity: ensure platform.GetHostsPath is reachable. The actual
	// return value depends on the OS but should never be empty.
	if platform.GetHostsPath() == "" {
		t.Fatal("platform.GetHostsPath() returned empty string")
	}
}

// isPermissionError is exported via the same package — test directly.
func TestIsPermissionError(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"bind: permission denied", true},
		{"open: EACCES", true},
		{"address already in use", false},
		{"network unreachable", false},
		{"", false},
	}
	for _, c := range cases {
		err := fmt.Errorf("%s", c.in)
		if got := isPermissionError(err); got != c.want {
			t.Fatalf("isPermissionError(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
