# Dev Plan: OpenHijack MVP (9 P0 Tasks)

**Reference**: @docs/openhijack-implementation-todo.md
**Allowed Backends**: codex, claude (UI falls back to codex)
**Coverage Requirement**: ≥90%

---

## Context & Constraints

- Go 1.26+ project, single binary, no external runtime deps
- Config: TOML via `github.com/BurntSushi/toml`
- CLI: stdlib `flag` package (not cobra)
- Logging: `log/slog`
- SecretStore abstraction already exists (`internal/crypto/store.go`)
- Provider enum already defined in config but only OpenAI Chat Completions is implemented
- GUI: Wails + Vue (no UI changes needed for P0 tasks)

## Codebase Exploration

| File | Role |
|------|------|
| `internal/proxy/transport.go` | `UpstreamTransport` - builds & forwards OpenAI requests, hardcoded `Bearer` auth |
| `internal/proxy/proxy.go` | `ProxyServer` - HTTP server, routing, SSE streaming, config loaded once at startup |
| `internal/config/config.go` | `Config`/`ConfigGroup` structs, `Load()`, Provider constants, no hot-reload |
| `internal/crypto/store.go` | `SecretStore` interface, `FileStore`, `EnvVarStore`, `GetGlobalStore()` |
| `cmd/openhijack/main.go` | CLI entry: `flag`-based commands (serve/init/cleanup/paths/elevate) |
| `internal/cert/cert.go` | `CertManager` - CA generation, server certs |
| `internal/hosts/hosts.go` | `HostsManager` - hosts file manipulation |

## Technical Decisions

1. **ProviderAdapter interface** in new `internal/proxy/provider/` package
2. **fsnotify** for config file watching (`github.com/fsnotify/fsnotify`)
3. **JSON Lines** format for audit logs (one JSON object per line)
4. **API Key ref**: `api_key_ref` field in ConfigGroup, resolved via SecretStore at startup
5. **Doctor command**: reuse existing `CertManager`, `HostsManager`, add health checks

## UI Determination
needs_ui: false
evidence: All P0 tasks are Go backend/CLI. GUI Vue files exist but no P0 task requires GUI changes.

---

## Task Breakdown

### Task 1: ProviderAdapter Framework + Anthropic + Gemini Adapters
- **ID**: t1_providers
- **Type**: default
- **Backend**: codex
- **Dependencies**: none
- **File Scope**:
  - NEW `internal/proxy/provider/provider.go` — ProviderAdapter interface + registry
  - NEW `internal/proxy/provider/openai.go` — OpenAI Chat Completions adapter (refactored from transport.go)
  - NEW `internal/proxy/provider/anthropic.go` — Anthropic Messages API adapter
  - NEW `internal/proxy/provider/gemini.go` — Gemini generateContent adapter
  - MODIFY `internal/proxy/transport.go` — refactor to use ProviderAdapter via registry
  - MODIFY `go.mod` — no new deps needed (uses stdlib)
- **Covers P0 tasks**: A1, A2, A3
- **Implementation Details**:
  - `ProviderAdapter` interface:
    ```go
    type ProviderAdapter interface {
        BuildRequest(ctx context.Context, group *config.ConfigGroup, body []byte, isStream bool) (*http.Request, error)
        TransformResponse(resp *http.Response) error
        // returns provider-specific auth headers
        AuthHeaders(group *config.ConfigGroup) http.Header
    }
    ```
  - Anthropic: `x-api-key` header, `anthropic-version: 2023-06-01`, `/v1/messages` endpoint, map OpenAI messages ↔ Anthropic messages
  - Gemini: `?key=` query param, `/v1beta/models/{model}:generateContent` endpoint, map OpenAI messages ↔ Gemini contents
  - Registry: `GetAdapter(provider string) ProviderAdapter`
  - transport.go: replace hardcoded OpenAI logic with `adapter.BuildRequest()` call
- **Test Command**: `go test ./internal/proxy/... -v -count=1`
- **Coverage Target**: ≥90% for new files

### Task 2: Provider Unit Tests
- **ID**: t2_provider_tests
- **Type**: default
- **Backend**: codex
- **Dependencies**: t1_providers
- **File Scope**:
  - NEW `internal/proxy/provider/provider_test.go` — registry tests
  - NEW `internal/proxy/provider/openai_test.go` — OpenAI adapter tests
  - NEW `internal/proxy/provider/anthropic_test.go` — Anthropic adapter tests (mock upstream)
  - NEW `internal/proxy/provider/gemini_test.go` — Gemini adapter tests (mock upstream)
- **Covers P0 task**: A4
- **Test Scenarios** (per provider):
  - Normal request → correct upstream URL, headers, body
  - Stream request → correct `stream` flag
  - Error response → proper error mapping
  - SSE response → correct stream passthrough
  - Model ID mapping
- **Test Command**: `go test ./internal/proxy/provider/... -v -cover -count=1`
- **Coverage Target**: ≥90%

### Task 3: Config Hot-Reload + Key Isolation + Audit Logging
- **ID**: t3_config_audit
- **Type**: default
- **Backend**: codex
- **Dependencies**: none
- **File Scope**:
  - NEW `internal/config/watcher.go` — fsnotify-based config file watcher
  - NEW `internal/audit/audit.go` — AuditLogger, JSON Lines writer
  - NEW `internal/audit/audit_test.go` — audit logger tests
  - MODIFY `internal/config/config.go` — add `APIKeyRef` field to ConfigGroup, add `Reload()` method
  - MODIFY `internal/proxy/proxy.go` — integrate watcher, add audit hooks in handleChatCompletions
  - MODIFY `go.mod` — add `github.com/fsnotify/fsnotify`
- **Covers P0 tasks**: B1, C1, C2
- **Implementation Details**:
  - **Hot-reload**: `ConfigWatcher` wraps fsnotify, calls `config.Load()` on change, atomic swap via `atomic.Pointer[Config]`, validation failure keeps old config
  - **Key isolation**: `ConfigGroup.APIKeyRef` field (toml: `api_key_ref`); when set, `ResolveAPIKey()` reads from SecretStore; `api_key` field is masked in CLI output
  - **Audit**: `AuditLogger` with `Log(entry AuditEntry)` method; `AuditEntry` struct: timestamp, request_id, model, config_group, status_code, token_usage, latency_ms, auth_key_id; writes to `<data_dir>/audit.log` as JSON Lines; `audit_enabled` config flag (default true)
- **Test Command**: `go test ./internal/config/... ./internal/audit/... -v -cover -count=1`
- **Coverage Target**: ≥90% for new files

### Task 4: Doctor Command + CI Workflow
- **ID**: t4_doctor_ci
- **Type**: quick-fix
- **Backend**: claude
- **Dependencies**: none
- **File Scope**:
  - MODIFY `cmd/openhijack/main.go` — add `doctor` subcommand
  - NEW `cmd/openhijack/doctor.go` — doctor logic (health checks)
  - NEW `cmd/openhijack/doctor_test.go` — doctor tests
  - NEW `.github/workflows/build.yml` — multi-platform CI
- **Covers P0 tasks**: D1, F1
- **Implementation Details**:
  - **Doctor checks**:
    1. CA cert exists (`CertManager.HasCA()`)
    2. CA cert trusted by system (attempt to verify)
    3. Hosts entries present (`HostsManager` check)
    4. Port 443 available (net.Listen test)
    5. Config file valid (`config.Load()`)
    6. Upstream reachable (HTTP HEAD to `api_url`)
  - Output: PASS/WARN/FAIL per check with fix suggestions
  - **CI**: matrix builds for ubuntu-latest, macos-latest, windows-latest; `go test ./... -cover`; `go build`
- **Test Command**: `go test ./cmd/openhijack/... -v -cover -count=1`
- **Coverage Target**: ≥90% for doctor logic

---

## Execution Order

```
t1_providers ──────→ t2_provider_tests
     │
     │ (parallel)
     │
t3_config_audit ──── (independent)
     │
t4_doctor_ci ─────── (independent)
```

- **Wave 1 (parallel)**: t1_providers, t3_config_audit, t4_doctor_ci
- **Wave 2 (after t1)**: t2_provider_tests

## Backend Routing Summary

| Task | Type | Preferred | Routed | Reason |
|------|------|-----------|--------|--------|
| t1_providers | default | codex | codex | in allowed_backends |
| t2_provider_tests | default | codex | codex | in allowed_backends |
| t3_config_audit | default | codex | codex | in allowed_backends |
| t4_doctor_ci | quick-fix | claude | claude | in allowed_backends |
