package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"openhijack/internal/config"
)

func TestAnthropicAdapter_GetUpstreamURL(t *testing.T) {
	adapter := &AnthropicAdapter{}

	group := &config.ConfigGroup{
		APIURL:      "https://api.anthropic.com",
		MiddleRoute: "",
	}

	url := adapter.GetUpstreamURL(group, false)
	if !strings.Contains(url, "/v1/messages") {
		t.Errorf("expected URL to contain /v1/messages, got %q", url)
	}

	if url != "https://api.anthropic.com/v1/messages" {
		t.Errorf("expected https://api.anthropic.com/v1/messages, got %q", url)
	}

	// With middle route
	groupMiddle := &config.ConfigGroup{
		APIURL:      "https://gateway.example.com",
		MiddleRoute: "/v1/anthropic",
	}
	urlMiddle := adapter.GetUpstreamURL(groupMiddle, false)
	if urlMiddle != "https://gateway.example.com/v1/anthropic/v1/messages" {
		t.Errorf("expected full URL with middle route, got %q", urlMiddle)
	}
}

func TestAnthropicAdapter_SetAuthHeaders(t *testing.T) {
	adapter := &AnthropicAdapter{}

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	group := &config.ConfigGroup{
		APIKey: "test-api-key-123",
	}

	adapter.SetAuthHeaders(req, group)

	if got := req.Header.Get("x-api-key"); got != "test-api-key-123" {
		t.Errorf("x-api-key = %q, want %q", got, "test-api-key-123")
	}

	if got := req.Header.Get("anthropic-version"); got != "2023-06-01" {
		t.Errorf("anthropic-version = %q, want %q", got, "2023-06-01")
	}

	// Empty API key should not set x-api-key
	req2 := httptest.NewRequest(http.MethodPost, "/test", nil)
	emptyKeyGroup := &config.ConfigGroup{APIKey: ""}
	adapter.SetAuthHeaders(req2, emptyKeyGroup)

	if got := req2.Header.Get("x-api-key"); got != "" {
		t.Errorf("with empty API key, x-api-key = %q, want empty", got)
	}
}

func TestAnthropicAdapter_BuildUpstreamRequest_NoSystem(t *testing.T) {
	adapter := &AnthropicAdapter{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// system field should be absent when no system messages
		if _, hasSystem := body["system"]; hasSystem {
			t.Errorf("unexpected system field in body")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"test","content":[],"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":10,"output_tokens":5}}`))
	}))
	defer ts.Close()

	group := &config.ConfigGroup{
		APIURL:      ts.URL,
		MiddleRoute: "",
		APIKey:      "sk-ant-test",
	}

	body := []byte(`{"messages":[{"role":"user","content":"Hello"}],"model":"claude-3-haiku-20240307"}`)

	req, err := adapter.BuildUpstreamRequest(context.Background(), group, "claude-3-5-sonnet-20241022", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest returned error: %v", err)
	}

	if req.URL.String() != ts.URL+"/v1/messages" {
		t.Errorf("URL = %q, want %q", req.URL.String(), ts.URL+"/v1/messages")
	}

	if got := req.Header.Get("x-api-key"); got != "sk-ant-test" {
		t.Errorf("x-api-key = %q, want %q", got, "sk-ant-test")
	}

	if got := req.Header.Get("anthropic-version"); got != "2023-06-01" {
		t.Errorf("anthropic-version = %q, want %q", got, "2023-06-01")
	}
}

func TestAnthropicAdapter_BuildUpstreamRequest_WithSystem(t *testing.T) {
	adapter := &AnthropicAdapter{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// system field should be present
		system, ok := body["system"].(string)
		if !ok || system == "" {
			t.Errorf("expected system field to be set, got system=%v ok=%v", body["system"], ok)
		}
		if system != "You are a helpful assistant." {
			t.Errorf("system = %q, want %q", system, "You are a helpful assistant.")
		}

		// messages should NOT contain the system message anymore
		messages, ok := body["messages"].([]interface{})
		if !ok {
			t.Fatalf("messages is not an array")
		}
		if len(messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(messages))
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"test","content":[],"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":10,"output_tokens":5}}`))
	}))
	defer ts.Close()

	group := &config.ConfigGroup{
		APIURL:      ts.URL,
		MiddleRoute: "",
		APIKey:      "sk-ant-test",
	}

	body := []byte(`{"messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hello"}],"model":"claude-3-haiku-20240307"}`)

	req, err := adapter.BuildUpstreamRequest(context.Background(), group, "claude-3-5-sonnet-20241022", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest returned error: %v", err)
	}

	if req.URL.String() != ts.URL+"/v1/messages" {
		t.Errorf("URL = %q, want %q", req.URL.String(), ts.URL+"/v1/messages")
	}
}

func TestAnthropicAdapter_BuildUpstreamRequest_Stream(t *testing.T) {
	adapter := &AnthropicAdapter{}

	group := &config.ConfigGroup{
		APIURL:      "https://api.anthropic.com",
		MiddleRoute: "",
	}

	// Input body already has stream=true; adapter should pass it through
	body := []byte(`{"messages":[{"role":"user","content":"Hi"}],"stream":true}`)
	req, err := adapter.BuildUpstreamRequest(context.Background(), group, "claude-3-haiku", body, true)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest returned error: %v", err)
	}

	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}

	// stream=true in input should be forwarded to output body
	var parsedBody map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&parsedBody); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}
	if stream, ok := parsedBody["stream"].(bool); !ok || !stream {
		t.Errorf("request body should have stream=true, got stream=%v", parsedBody["stream"])
	}
}

func TestAnthropicAdapter_BuildUpstreamRequest_NilCtx(t *testing.T) {
	adapter := &AnthropicAdapter{}

	group := &config.ConfigGroup{
		APIURL:      "https://api.anthropic.com",
		MiddleRoute: "",
		APIKey:      "sk-ant-test",
	}

	// nil ctx should not panic
	req, err := adapter.BuildUpstreamRequest(nil, group, "claude-3-haiku", []byte(`{"messages":[{"role":"user","content":"Hi"}]}`), false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest with nil ctx returned error: %v", err)
	}
	if req == nil {
		t.Fatal("expected non-nil request with nil ctx")
	}
}

func TestAnthropicAdapter_BuildUpstreamRequest_InvalidJSON(t *testing.T) {
	adapter := &AnthropicAdapter{}

	group := &config.ConfigGroup{
		APIURL:      "https://api.anthropic.com",
		MiddleRoute: "",
		APIKey:      "sk-ant-test",
	}

	// Invalid JSON body should return an error
	_, err := adapter.BuildUpstreamRequest(context.Background(), group, "claude-3-haiku", []byte("not json"), false)
	if err == nil {
		t.Fatal("expected error for invalid JSON body, got nil")
	}
}

func TestAnthropicAdapter_BuildUpstreamRequest_CustomHeaders(t *testing.T) {
	adapter := &AnthropicAdapter{}

	group := &config.ConfigGroup{
		APIURL:      "https://api.anthropic.com",
		MiddleRoute: "",
		APIKey:      "sk-ant-test",
		Headers: map[string]string{
			"X-Custom":     "my-custom-value",
			"X-Forward-For": "1.2.3.4",
		},
	}

	body := []byte(`{"messages":[{"role":"user","content":"Hi"}]}`)
	req, err := adapter.BuildUpstreamRequest(context.Background(), group, "claude-3-haiku", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest returned error: %v", err)
	}

	if got := req.Header.Get("X-Custom"); got != "my-custom-value" {
		t.Errorf("X-Custom = %q, want %q", got, "my-custom-value")
	}
	if got := req.Header.Get("X-Forward-For"); got != "1.2.3.4" {
		t.Errorf("X-Forward-For = %q, want %q", got, "1.2.3.4")
	}
}

// --- MapResponseToOpenAI tests ---

func TestAnthropicAdapter_MapResponseToOpenAI_Normal(t *testing.T) {
	adapter := &AnthropicAdapter{}

	// Simulate a real Anthropic /v1/messages response
	input := []byte(`{
		"id": "msg_123",
		"type": "message",
		"model": "claude-3-5-sonnet-20241022",
		"role": "assistant",
		"content": [
			{"type": "text", "text": "Hello, this is a test response."},
			{"type": "text", "text": "Second paragraph."}
		],
		"stop_reason": "end_turn",
		"stop_sequence": null,
		"usage": {"input_tokens": 10, "output_tokens": 25}
	}`)

	output, err := adapter.MapResponseToOpenAI(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse the output to verify structure
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Verify top-level fields
	if result["object"] != "chat.completion" {
		t.Errorf("object = %q, want %q", result["object"], "chat.completion")
	}
	if result["id"] != "msg_123" {
		t.Errorf("id = %q, want %q", result["id"], "msg_123")
	}
	if result["model"] != "claude-3-5-sonnet-20241022" {
		t.Errorf("model = %q, want %q", result["model"], "claude-3-5-sonnet-20241022")
	}

	// Verify choices[0].message.content
	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatal("expected non-empty choices array")
	}
	choice0, ok := choices[0].(map[string]interface{})
	if !ok {
		t.Fatal("choices[0] is not an object")
	}
	msg, ok := choice0["message"].(map[string]interface{})
	if !ok {
		t.Fatal("message is not an object")
	}
	if msg["role"] != "assistant" {
		t.Errorf("message.role = %q, want %q", msg["role"], "assistant")
	}
	content, ok := msg["content"].(string)
	if !ok {
		t.Fatal("content is not a string")
	}
	wantContent := "Hello, this is a test response.Second paragraph."
	if content != wantContent {
		t.Errorf("content = %q, want %q", content, wantContent)
	}
	if choice0["finish_reason"] != "stop" {
		t.Errorf("finish_reason = %q, want %q", choice0["finish_reason"], "stop")
	}

	// Verify usage
	usage, ok := result["usage"].(map[string]interface{})
	if !ok {
		t.Fatal("usage is not an object")
	}
	if int64(usage["prompt_tokens"].(float64)) != 10 {
		t.Errorf("prompt_tokens = %v, want 10", usage["prompt_tokens"])
	}
	if int64(usage["completion_tokens"].(float64)) != 25 {
		t.Errorf("completion_tokens = %v, want 25", usage["completion_tokens"])
	}
}

func TestAnthropicAdapter_MapResponseToOpenAI_InvalidJSON(t *testing.T) {
	adapter := &AnthropicAdapter{}

	// MapResponseToOpenAI returns the original body unchanged (not an error) when JSON is invalid
	invalidInput := []byte("not json")
	output, err := adapter.MapResponseToOpenAI(invalidInput)
	if err != nil {
		t.Fatalf("expected nil error for invalid JSON, got: %v", err)
	}
	// The function returns the input body as-is when unmarshal fails
	if string(output) != string(invalidInput) {
		t.Errorf("output = %q, want input returned unchanged %q", string(output), string(invalidInput))
	}
}

func TestAnthropicAdapter_MapResponseToOpenAI_StopReasonToolUse(t *testing.T) {
	adapter := &AnthropicAdapter{}

	// stop_reason "tool_use" should map to finish_reason "tool_calls"
	input := []byte(`{
		"id": "msg_tool",
		"type": "message",
		"model": "claude-3-5-sonnet-20241022",
		"content": [{"type": "text", "text": "Calling tool..."}],
		"stop_reason": "tool_use",
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`)

	output, err := adapter.MapResponseToOpenAI(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatal("expected non-empty choices")
	}
	choice0, ok := choices[0].(map[string]interface{})
	if !ok {
		t.Fatal("choices[0] is not an object")
	}
	if choice0["finish_reason"] != "tool_calls" {
		t.Errorf("finish_reason = %q, want %q", choice0["finish_reason"], "tool_calls")
	}
}

func TestAnthropicAdapter_MapResponseToOpenAI_UnknownStopReason(t *testing.T) {
	adapter := &AnthropicAdapter{}

	// Unknown stop_reason should pass through as-is (default branch)
	input := []byte(`{
		"id": "msg_unknown",
		"type": "message",
		"model": "claude-3-haiku",
		"content": [{"type": "text", "text": "done"}],
		"stop_reason": "unknown_reason",
		"usage": {"input_tokens": 3, "output_tokens": 2}
	}`)

	output, err := adapter.MapResponseToOpenAI(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatal("expected non-empty choices")
	}
	choice0, ok := choices[0].(map[string]interface{})
	if !ok {
		t.Fatal("choices[0] is not an object")
	}
	if choice0["finish_reason"] != "unknown_reason" {
		t.Errorf("finish_reason = %q, want %q", choice0["finish_reason"], "unknown_reason")
	}
}

func TestAnthropicAdapter_MapResponseToOpenAI_EmptyContent(t *testing.T) {
	adapter := &AnthropicAdapter{}

	// Response with empty content array — should not panic
	input := []byte(`{
		"id": "msg_empty",
		"type": "message",
		"model": "claude-3-haiku",
		"content": [],
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 5, "output_tokens": 0}
	}`)

	output, err := adapter.MapResponseToOpenAI(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatal("expected non-empty choices array")
	}
	choice0, ok := choices[0].(map[string]interface{})
	if !ok {
		t.Fatal("choices[0] is not an object")
	}
	msg, ok := choice0["message"].(map[string]interface{})
	if !ok {
		t.Fatal("message is not an object")
	}
	// content should be empty string, not nil or missing
	content, ok := msg["content"].(string)
	if !ok {
		t.Fatal("content is not a string")
	}
	if content != "" {
		t.Errorf("content = %q, want empty string", content)
	}
}

// --- MapStreamToOpenAI tests ---

func TestAnthropicAdapter_MapStreamToOpenAI_Normal(t *testing.T) {
	adapter := &AnthropicAdapter{}

	// Simulate an Anthropic SSE stream
	sseInput := `event: message_start
data: {"type":"message","id":"msg_stream","model":"claude-3-5-sonnet-20241022","role":"assistant","content":[],"stop_reason":null}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text"," world"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null}}

`

	var buf bytes.Buffer
	err := adapter.MapStreamToOpenAI(strings.NewReader(sseInput), &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Verify we got OpenAI-format data lines
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var dataLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, line)
		}
	}

	if len(dataLines) < 2 {
		t.Fatalf("expected at least 2 data lines, got %d:\n%s", len(dataLines), output)
	}

	// First data line should be a chat.completion.chunk with text content
	var firstChunk map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(dataLines[0], "data: ")), &firstChunk); err != nil {
		t.Fatalf("first data line is not valid JSON: %v", err)
	}
	if firstChunk["object"] != "chat.completion.chunk" {
		t.Errorf("first chunk object = %q, want %q", firstChunk["object"], "chat.completion.chunk")
	}
	choices, ok := firstChunk["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatal("expected non-empty choices in first chunk")
	}
	choice0, ok := choices[0].(map[string]interface{})
	if !ok {
		t.Fatal("choices[0] is not an object")
	}
	delta, ok := choice0["delta"].(map[string]interface{})
	if !ok {
		t.Fatal("delta is not an object")
	}
	if delta["role"] != "assistant" {
		t.Errorf("delta.role = %q, want %q", delta["role"], "assistant")
	}
	if delta["content"] != "Hello" {
		t.Errorf("delta.content = %q, want %q", delta["content"], "Hello")
	}
	if completionID, ok := firstChunk["id"].(string); !ok || completionID == "" {
		t.Error("first chunk should have a non-empty id from message_start")
	}

	// Last data line should be [DONE]
	lastLine := dataLines[len(dataLines)-1]
	if lastLine != "data: [DONE]" {
		t.Errorf("last data line = %q, want %q", lastLine, "data: [DONE]")
	}
}

func TestAnthropicAdapter_MapStreamToOpenAI_EmptyInput(t *testing.T) {
	adapter := &AnthropicAdapter{}

	// Empty input — should not panic
	var buf bytes.Buffer
	err := adapter.MapStreamToOpenAI(strings.NewReader(""), &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No output expected for empty input
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

func TestAnthropicAdapter_MapStreamToOpenAI_MaxTokensStopReason(t *testing.T) {
	adapter := &AnthropicAdapter{}

	// Test that max_tokens stop_reason maps to "length"
	sseInput := `event: message_start
data: {"type":"message","id":"msg_mt","model":"claude-3-haiku","role":"assistant","content":[],"stop_reason":null}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"partial answer"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"max_tokens","stop_sequence":null}}

`

	var buf bytes.Buffer
	err := adapter.MapStreamToOpenAI(strings.NewReader(sseInput), &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var dataLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, line)
		}
	}

	// Find the message_delta chunk (second data line before [DONE])
	if len(dataLines) < 2 {
		t.Fatalf("expected at least 2 data lines, got %d", len(dataLines))
	}

	// The second-to-last should be the finish_reason chunk
	finishLine := dataLines[len(dataLines)-2]
	var finishChunk map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(finishLine, "data: ")), &finishChunk); err != nil {
		t.Fatalf("finish chunk is not valid JSON: %v", err)
	}
	choices, ok := finishChunk["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatal("expected non-empty choices in finish chunk")
	}
	choice0, ok := choices[0].(map[string]interface{})
	if !ok {
		t.Fatal("choices[0] is not an object")
	}
	if choice0["finish_reason"] != "length" {
		t.Errorf("finish_reason = %q, want %q", choice0["finish_reason"], "length")
	}
}
