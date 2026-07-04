package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// AuditEntry describes a single request-level audit record.
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"request_id"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Status    int       `json:"status"`
	Upstream  string    `json:"upstream,omitempty"`
	Model     string    `json:"model,omitempty"`
	Duration  string    `json:"duration_ms"` // millisecond-precision duration
	ClientIP  string    `json:"client_ip,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// AuditLogger writes JSON Lines audit records to an io.Writer.
// Each Log call produces one JSON object followed by a newline.
type AuditLogger struct {
	mu sync.Mutex
	w  io.Writer
}

// NewAuditLogger creates a new AuditLogger that writes to w.
func NewAuditLogger(w io.Writer) *AuditLogger {
	return &AuditLogger{w: w}
}

// Log writes a single AuditEntry as a JSON Lines record.
func (a *AuditLogger) Log(entry AuditEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.w == nil {
		return nil
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = a.w.Write(data)
	if err != nil {
		return err
	}

	_, err = a.w.Write([]byte("\n"))
	return err
}

// LogRequest is a convenience helper that builds an AuditEntry from
// common request parameters and calls Log.
func (a *AuditLogger) LogRequest(requestID, method, path string, status int, upstream, model, clientIP string, duration time.Duration, errMsg string) error {
	return a.Log(AuditEntry{
		Timestamp: time.Now(),
		RequestID: requestID,
		Method:    method,
		Path:      path,
		Status:    status,
		Upstream:  upstream,
		Model:     model,
		Duration:  formatDuration(duration),
		ClientIP:  clientIP,
		Error:     errMsg,
	})
}

// formatDuration converts a time.Duration to a string with millisecond
// precision (e.g. "123.45").
func formatDuration(d time.Duration) string {
	ms := float64(d.Microseconds()) / 1000.0
	return fmt.Sprintf("%.2f", ms)
}
