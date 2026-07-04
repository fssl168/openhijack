// Package health provides shared health-check logic used by both the
// `openhijack doctor` CLI command and the Wails GUI dashboard.
//
// The checks are intentionally side-effect free: they only read state
// (files, ports, network) and report status. Any fixing actions remain
// the responsibility of the caller.
package health

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"openhijack/internal/cert"
	"openhijack/internal/config"
	"openhijack/internal/hosts"
	"openhijack/internal/platform"
)

// Check status constants. These are stable strings (not iota) so they
// round-trip cleanly through JSON to TypeScript unions.
const (
	StatusPass = "PASS"
	StatusWarn = "WARN"
	StatusFail = "FAIL"
)

// DefaultPort is the port checked by CheckPortAvailability.
const DefaultPort = 443

// CheckResult holds the outcome of a single health check.
type CheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // PASS | WARN | FAIL
	Detail  string `json:"detail"`
	FixHint string `json:"fix_hint,omitempty"`
}

// Options controls which checks RunAllChecks performs and where it
// looks for state. Zero-value fields fall back to defaults.
type Options struct {
	DataDir     string
	ConfigPath  string
	Port        int    // 0 → DefaultPort
	UpstreamTimeout time.Duration // 0 → 10s
}

// RunAllChecks executes every health check in sequence and returns
// the combined results. The order is stable: CA → ServerCert → Hosts
// → Port → Config → Upstream. This matches the CLI doctor report.
func RunAllChecks(opts Options) []CheckResult {
	if opts.Port == 0 {
		opts.Port = DefaultPort
	}
	if opts.UpstreamTimeout == 0 {
		opts.UpstreamTimeout = 10 * time.Second
	}

	results := make([]CheckResult, 0, 6)
	results = append(results, CheckCACertificate(opts.DataDir))
	results = append(results, CheckServerCert(opts.DataDir))
	results = append(results, CheckHostsEntries())
	results = append(results, CheckPortAvailability(opts.Port))
	results = append(results, CheckConfigFile(opts.ConfigPath))
	results = append(results, CheckUpstreamReachability(opts.ConfigPath, opts.UpstreamTimeout))
	return results
}

// Summary counts PASS/WARN/FAIL results.
func Summary(results []CheckResult) (pass, warn, fail int) {
	for _, r := range results {
		switch r.Status {
		case StatusPass:
			pass++
		case StatusWarn:
			warn++
		case StatusFail:
			fail++
		}
	}
	return
}

// CheckCACertificate verifies that the CA certificate and key exist.
func CheckCACertificate(dataDir string) CheckResult {
	cm := cert.NewCertManager(dataDir)
	if cm.HasCA() {
		return CheckResult{
			Name:   "CA Certificate",
			Status: StatusPass,
			Detail: fmt.Sprintf("Found at %s", cm.CACertFile()),
		}
	}
	return CheckResult{
		Name:    "CA Certificate",
		Status:  StatusFail,
		Detail:  "CA certificate not found",
		FixHint: "Run 'openhijack elevate' to generate certificates",
	}
}

// CheckServerCert verifies that the server certificate exists.
func CheckServerCert(dataDir string) CheckResult {
	cm := cert.NewCertManager(dataDir)
	if cm.HasServerCert() {
		return CheckResult{
			Name:   "Server Certificate",
			Status: StatusPass,
			Detail: "Found",
		}
	}
	return CheckResult{
		Name:    "Server Certificate",
		Status:  StatusFail,
		Detail:  "Server certificate not found",
		FixHint: "Run 'openhijack elevate' to generate certificates",
	}
}

// CheckHostsEntries verifies that OpenHijack domains are present in
// the hosts file. It reads the system hosts path via
// platform.GetHostsPath.
func CheckHostsEntries() CheckResult {
	return CheckHostsEntriesAt(platform.GetHostsPath())
}

// CheckHostsEntriesAt is the path-parameterized variant of
// CheckHostsEntries, exposed for testing and for callers that already
// know the hosts file path (e.g. when running inside a chroot).
func CheckHostsEntriesAt(hostsFile string) CheckResult {
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return CheckResult{
			Name:    "Hosts Entries",
			Status:  StatusFail,
			Detail:  fmt.Sprintf("Cannot read hosts file (%s): %v", hostsFile, err),
			FixHint: "Run 'openhijack elevate' to add hosts entries",
		}
	}

	body := string(content)

	if !strings.Contains(body, hosts.Markers()) {
		return CheckResult{
			Name:    "Hosts Entries",
			Status:  StatusFail,
			Detail:  "No OpenHijack entries found",
			FixHint: "Run 'openhijack elevate' to add hosts entries",
		}
	}

	domains := hosts.Domains()
	found := 0
	for _, domain := range domains {
		if strings.Contains(body, domain) {
			found++
		}
	}

	if found == len(domains) {
		return CheckResult{
			Name:   "Hosts Entries",
			Status: StatusPass,
			Detail: fmt.Sprintf("All domains mapped (%s)", strings.Join(domains, ", ")),
		}
	}

	return CheckResult{
		Name:    "Hosts Entries",
		Status:  StatusWarn,
		Detail:  fmt.Sprintf("Partial: %d/%d domains mapped", found, len(domains)),
		FixHint: "Run 'openhijack elevate' to add missing hosts entries",
	}
}

// CheckPortAvailability tests whether the given port is bindable.
func CheckPortAvailability(port int) CheckResult {
	addr := fmt.Sprintf(":%d", port)
	conn, err := net.Listen("tcp", addr)
	if err != nil {
		if isPermissionError(err) {
			return CheckResult{
				Name:    fmt.Sprintf("Port %d", port),
				Status:  StatusFail,
				Detail:  "Permission denied (port < 1024 requires root or capability)",
				FixHint: "Run with 'sudo' or use a non-privileged port (--port 8443)",
			}
		}
		return CheckResult{
			Name:   fmt.Sprintf("Port %d", port),
			Status: StatusWarn,
			Detail: "Port already in use (OpenHijack may be running)",
		}
	}
	conn.Close()
	return CheckResult{
		Name:   fmt.Sprintf("Port %d", port),
		Status: StatusPass,
		Detail: "Available",
	}
}

// CheckConfigFile attempts to load the config and reports status.
func CheckConfigFile(configPath string) CheckResult {
	cfg, err := config.Load(configPath)
	if err != nil {
		return CheckResult{
			Name:    "Config File",
			Status:  StatusFail,
			Detail:  fmt.Sprintf("Cannot load config: %v", err),
			FixHint: fmt.Sprintf("Run 'openhijack init' and edit %s", configPath),
		}
	}
	g := cfg.CurrentGroup()
	return CheckResult{
		Name:   "Config File",
		Status: StatusPass,
		Detail: fmt.Sprintf("Valid — provider=%q model=%q", g.Provider, g.ModelID),
	}
}

// CheckUpstreamReachability sends an HTTP HEAD to the configured
// api_url to verify network connectivity.
func CheckUpstreamReachability(configPath string, timeout time.Duration) CheckResult {
	cfg, err := config.Load(configPath)
	if err != nil {
		return CheckResult{
			Name:   "Upstream Reachability",
			Status: StatusPass,
			Detail: "Skipped (config invalid)",
		}
	}

	apiURL := cfg.CurrentGroup().APIURL
	if apiURL == "" {
		return CheckResult{
			Name:   "Upstream Reachability",
			Status: StatusWarn,
			Detail: "api_url not configured",
		}
	}

	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	req, err := http.NewRequest(http.MethodHead, apiURL, nil)
	if err != nil {
		return CheckResult{
			Name:    "Upstream Reachability",
			Status:  StatusFail,
			Detail:  fmt.Sprintf("Invalid api_url: %v", err),
			FixHint: "Check api_url in config",
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return CheckResult{
			Name:    "Upstream Reachability",
			Status:  StatusWarn,
			Detail:  fmt.Sprintf("Connection timeout/unreachable (%s)", apiURL),
			FixHint: "Check api_url in config and network connectivity",
		}
	}
	defer resp.Body.Close()

	return CheckResult{
		Name:   "Upstream Reachability",
		Status: StatusPass,
		Detail: fmt.Sprintf("Upstream responded with HTTP %d (%s)", resp.StatusCode, apiURL),
	}
}

// isPermissionError reports whether an error is a permission-denied
// error. Used by CheckPortAvailability.
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "permission") ||
		strings.Contains(msg, "EACCES")
}
