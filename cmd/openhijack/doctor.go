package main

import (
	"fmt"
	"os"

	"openhijack/internal/health"
)

// doctorResult is an alias for health.CheckResult exposed via the
// health package. We keep the local alias so the GUI's RunDoctor
// binding (which imports gui/app.go) can return a stable type
// without coupling the CLI to internal/health types directly.
type doctorResult = health.CheckResult

// runDoctor executes all health checks and prints a summary report.
func runDoctor() {
	results := runAllChecks()
	printReport(results)
}

// runAllChecks delegates to the shared health package, computing the
// data dir and config path the same way the CLI always has.
func runAllChecks() []health.CheckResult {
	return health.RunAllChecks(health.Options{
		DataDir:    getDataDir(),
		ConfigPath: resolveConfigPath(""),
	})
}

// printReport renders the check results as a human-readable report.
// This function is CLI-only; the GUI uses structured results.
func printReport(results []health.CheckResult) {
	fmt.Println("OpenHijack Doctor - Health Check Report")
	fmt.Println("========================================")
	fmt.Println()

	pass, warn, fail := health.Summary(results)

	for _, r := range results {
		fmt.Printf("[%s] %s: %s", r.Status, r.Name, r.Detail)
		if r.FixHint != "" {
			fmt.Printf("\n       Fix: %s", r.FixHint)
		}
		fmt.Println()
	}

	fmt.Println()
	fmt.Printf("Summary: %d passed, %d warning, %d failure\n", pass, warn, fail)

	if fail > 0 {
		os.Exit(1)
	}
}
