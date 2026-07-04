package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"openhijack/internal/config"
)

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func newGroup(apiURL, modelID, apiKey string) *config.ConfigGroup {
	return &config.ConfigGroup{
		APIURL:  apiURL,
		ModelID: modelID,
		APIKey:  apiKey,
	}
}

func TestGeminiAdapter_GetUpstreamURL_NonStream(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := newGroup("https://generativelanguage.googleapis.com", "gemini-2.0-flash", "")

	got := adapter.GetUpstreamURL(group, false)

	if !contains(got, ":generateContent") {
		t.Errorf("URL should contain :generateContent, got %q", got)
	}
	if !contains(got, "gemini-2.0-flash") {
		t.Errorf("URL should contain model id, got %q", got)
	}
}

func TestGeminiAdapter_GetUpstreamURL_Stream(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := newGroup("https://generativelanguage.googleapis.com", "gemini-2.0-flash", "")

	got := adapter.GetUpstreamURL(group, true)

	if !contains(got, ":streamGenerateContent") {
		t.Errorf("URL should contain :streamGenerateContent, got %q", got)
	}
	if !contains(got, "alt=sse") {
		t.Errorf("URL should contain alt=sse, got %q", got)
	}
}

func TestGeminiAdapter_SetAuthHeaders(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := newGroup("", "", "test-api-key-123")

	req, err := http.NewRequest("GET", "https://example.com/v1beta/models/test", nil)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	adapter.SetAuthHeaders(req, group)

	q := req.URL.Query()
	if q.Get("key") != "test-api-key-123" {
		t.Errorf("query key should be 'test-api-key-123', got %q", q.Get("key"))
	}
}

func TestGeminiAdapter_BuildUpstreamRequest_Normal(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := &config.ConfigGroup{
		APIURL:  "https://generativelanguage.googleapis.com",
		ModelID: "gemini-pro",
		APIKey:  "test-key",
	}

	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": "You are helpful."},
			{"role": "user", "content": "Hello"}
		],
		"temperature": 0.7
	}`)

	req, err := adapter.BuildUpstreamRequest(context.Background(), group, "gemini-pro", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest failed: %v", err)
	}

	// Check URL contains target model
	if !contains(req.URL.String(), "gemini-pro") {
		t.Errorf("URL should contain model id, got %q", req.URL.String())
	}

	// Check URL contains :generateContent
	if !contains(req.URL.String(), ":generateContent") {
		t.Errorf("URL should contain :generateContent, got %q", req.URL.String())
	}

	// Check Content-Type header
	if ct := req.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type should be 'application/json', got %q", ct)
	}

	// Check body has "contents" field
	rawBody, _ := io.ReadAll(req.Body)
	var parsed map[string]interface{}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	contents, ok := parsed["contents"].([]interface{})
	if !ok || len(contents) == 0 {
		t.Errorf("request body should have non-empty contents array")
	}

	// system message should appear as systemInstruction
	if _, hasSystem := parsed["systemInstruction"]; !hasSystem {
		t.Error("request body should have systemInstruction when system message present")
	}

	// Check generation config
	cfg, ok := parsed["generationConfig"].(map[string]interface{})
	if !ok {
		t.Error("request body should have generationConfig")
	} else if temp := cfg["temperature"]; temp != 0.7 {
		t.Errorf("generationConfig.temperature should be 0.7, got %v", temp)
	}
}

func TestGeminiAdapter_BuildUpstreamRequest_NilCtx(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := &config.ConfigGroup{
		APIURL:  "https://generativelanguage.googleapis.com",
		ModelID: "gemini-pro",
	}

	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`)

	// Should not panic with nil context
	req, err := adapter.BuildUpstreamRequest(nil, group, "gemini-pro", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest with nil ctx should not error, got: %v", err)
	}
	if req == nil {
		t.Fatal("expected non-nil request with nil ctx")
	}
}

func TestGeminiAdapter_BuildUpstreamRequest_InvalidJSON(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := &config.ConfigGroup{
		APIURL:  "https://generativelanguage.googleapis.com",
		ModelID: "gemini-pro",
	}

	body := []byte("not json")

	_, err := adapter.BuildUpstreamRequest(context.Background(), group, "gemini-pro", body, false)
	if err == nil {
		t.Fatal("expected error for invalid JSON body")
	}
}

func TestGeminiAdapter_MapResponseToOpenAI_Normal(t *testing.T) {
	adapter := &GeminiAdapter{}

	geminiJSON := []byte(`{
		"candidates": [{
			"content": {
				"modelVersion": "gemini-2.0-flash",
				"parts": [{"text": "Hello, how can I help you?"}]
			},
			"finishReason": "STOP",
			"index": 0
		}],
		"usageMetadata": {
			"promptTokenCount": 10,
			"candidatesTokenCount": 25,
			"totalTokenCount": 35
		},
		"name": "models/gemini-2.0-flash"
	}`)

	out, err := adapter.MapResponseToOpenAI(geminiJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Verify top-level OpenAI fields
	if result["object"] != "chat.completion" {
		t.Errorf("expected object=chat.completion, got %v", result["object"])
	}
	if result["model"] != "gemini-2.0-flash" {
		t.Errorf("expected model=gemini-2.0-flash, got %v", result["model"])
	}

	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatalf("expected non-empty choices array")
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		t.Fatalf("choices[0] is not an object")
	}

	msg, ok := choice["message"].(map[string]interface{})
	if !ok {
		t.Fatalf("message is not an object")
	}
	if msg["role"] != "assistant" {
		t.Errorf("expected role=assistant, got %v", msg["role"])
	}
	if msg["content"] != "Hello, how can I help you?" {
		t.Errorf("expected content='Hello, how can I help you?', got %v", msg["content"])
	}

	if choice["finish_reason"] != "stop" {
		t.Errorf("expected finish_reason=stop, got %v", choice["finish_reason"])
	}

	usage, ok := result["usage"].(map[string]interface{})
	if !ok {
		t.Fatalf("usage is not an object")
	}
	if int64(usage["prompt_tokens"].(float64)) != 10 {
		t.Errorf("expected prompt_tokens=10, got %v", usage["prompt_tokens"])
	}
	if int64(usage["completion_tokens"].(float64)) != 25 {
		t.Errorf("expected completion_tokens=25, got %v", usage["completion_tokens"])
	}
	if int64(usage["total_tokens"].(float64)) != 35 {
		t.Errorf("expected total_tokens=35, got %v", usage["total_tokens"])
	}
}

func TestGeminiAdapter_MapResponseToOpenAI_InvalidJSON(t *testing.T) {
	adapter := &GeminiAdapter{}

	input := []byte("not json")
	out, err := adapter.MapResponseToOpenAI(input)
	if err != nil {
		t.Fatalf("expected no error for invalid JSON, got: %v", err)
	}

	// For invalid JSON, the function returns the original body unchanged
	if string(out) != string(input) {
		t.Errorf("expected original body returned, got %q", string(out))
	}
}

func TestGeminiAdapter_MapResponseToOpenAI_NoCandidates(t *testing.T) {
	adapter := &GeminiAdapter{}

	// No "candidates" field at all
	input := []byte(`{"usageMetadata": {"promptTokenCount": 5, "candidatesTokenCount": 10, "totalTokenCount": 15}}`)
	out, err := adapter.MapResponseToOpenAI(input)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should fall through and return original body (empty candidates → fallback path)
	if string(out) != string(input) {
		t.Errorf("expected original body returned for missing candidates, got %q", string(out))
	}

	// candidates that is an empty array
	inputEmpty := []byte(`{"candidates": []}`)
	out2, err := adapter.MapResponseToOpenAI(inputEmpty)
	if err != nil {
		t.Fatalf("expected no error for empty candidates, got: %v", err)
	}
	if string(out2) != string(inputEmpty) {
		t.Errorf("expected original body returned for empty candidates, got %q", string(out2))
	}
}

func TestGeminiAdapter_MapStreamToOpenAI_Normal(t *testing.T) {
	adapter := &GeminiAdapter{}

	// Build a minimal Gemini SSE stream with two chunks (first + final STOP)
	geminiSSE := `data: {"candidates":[{"content":{"parts":[{"text":"Hello"}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":10,"totalTokenCount":15}}

data: [DONE]

`

	var buf bytes.Buffer
	err := adapter.MapStreamToOpenAI(strings.NewReader(geminiSSE), &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// The output should contain at least one "data:" line with OpenAI chunk format
	if !strings.Contains(output, "data:") {
		t.Fatal("expected output to contain data: lines")
	}
	if !strings.Contains(output, "chat.completion.chunk") {
		t.Fatal("expected output to contain chat.completion.chunk")
	}
	if !strings.Contains(output, "chatcmpl-gemini") {
		t.Fatal("expected output to contain chatcmpl-gemini")
	}
	if !strings.Contains(output, "\"role\":\"assistant\"") && !strings.Contains(output, "\"role\": \"assistant\"") {
		t.Fatal("expected output to contain assistant role")
	}
	if !strings.Contains(output, "\"content\":\"Hello\"") && !strings.Contains(output, "\"content\": \"Hello\"") {
		t.Fatal("expected output to contain Hello content")
	}
	if !strings.Contains(output, "[DONE]") {
		t.Fatal("expected output to contain [DONE]")
	}
}

func TestGeminiAdapter_MapStreamToOpenAI_EmptyInput(t *testing.T) {
	adapter := &GeminiAdapter{}

	// Empty reader should not panic
	var buf bytes.Buffer
	err := adapter.MapStreamToOpenAI(strings.NewReader(""), &buf)
	if err != nil {
		t.Fatalf("unexpected error for empty input: %v", err)
	}

	output := buf.String()
	// No SSE data lines for empty input
	if strings.Contains(output, "data:") {
		t.Errorf("expected no data lines for empty input, got: %q", output)
	}
}

func TestGeminiAdapter_BuildUpstreamRequest_WithGenerationConfig(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := &config.ConfigGroup{
		APIURL:  "https://generativelanguage.googleapis.com",
		ModelID: "gemini-pro",
		APIKey:  "test-key",
	}

	body := []byte(`{
		"model": "gpt-4",
		"messages": [{"role": "user", "content": "hi"}],
		"temperature": 0.9,
		"top_p": 0.5,
		"max_tokens": 256
	}`)

	req, err := adapter.BuildUpstreamRequest(context.Background(), group, "gemini-pro", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest failed: %v", err)
	}

	rawBody, _ := io.ReadAll(req.Body)
	var parsed map[string]interface{}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	cfg, ok := parsed["generationConfig"].(map[string]interface{})
	if !ok {
		t.Fatal("expected generationConfig in request body")
	}

	if cfg["temperature"] != 0.9 {
		t.Errorf("generationConfig.temperature = %v, want 0.9", cfg["temperature"])
	}
	if cfg["topP"] != 0.5 {
		t.Errorf("generationConfig.topP = %v, want 0.5", cfg["topP"])
	}
	if cfg["maxOutputTokens"] != float64(256) {
		t.Errorf("generationConfig.maxOutputTokens = %v, want 256", cfg["maxOutputTokens"])
	}
}

func TestGeminiAdapter_BuildUpstreamRequest_ArrayContent(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := &config.ConfigGroup{
		APIURL:  "https://generativelanguage.googleapis.com",
		ModelID: "gemini-pro",
		APIKey:  "test-key",
	}

	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": ["Hello", "World"]},
			{"role": "assistant", "content": "Hi there"}
		]
	}`)

	req, err := adapter.BuildUpstreamRequest(context.Background(), group, "gemini-pro", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest failed: %v", err)
	}

	rawBody, _ := io.ReadAll(req.Body)
	var parsed map[string]interface{}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	contents, ok := parsed["contents"].([]interface{})
	if !ok || len(contents) != 2 {
		t.Fatalf("expected 2 contents, got %d", len(contents))
	}

	// user message: array content joined with \n
	userContent, ok := contents[0].(map[string]interface{})
	if !ok {
		t.Fatal("first content is not a map")
	}
	parts, ok := userContent["parts"].([]interface{})
	if !ok || len(parts) != 1 {
		t.Fatalf("expected 1 part in user content, got %d", len(parts))
	}
	partMap, ok := parts[0].(map[string]interface{})
	if !ok {
		t.Fatal("part is not a map")
	}
	if partMap["text"] != "Hello\nWorld" {
		t.Errorf("user text = %v, want %q", partMap["text"], "Hello\nWorld")
	}

	// assistant message should map to "model" role
	modelContent, ok := contents[1].(map[string]interface{})
	if !ok {
		t.Fatal("second content is not a map")
	}
	if modelContent["role"] != "model" {
		t.Errorf("model role = %v, want %q", modelContent["role"], "model")
	}
}

func TestGeminiAdapter_BuildUpstreamRequest_NestedTextContent(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := &config.ConfigGroup{
		APIURL:  "https://generativelanguage.googleapis.com",
		ModelID: "gemini-pro",
		APIKey:  "test-key",
	}

	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Hello"}, {"type": "text", "text": "World"}]},
			{"role": "assistant", "content": "Response"}
		]
	}`)

	req, err := adapter.BuildUpstreamRequest(context.Background(), group, "gemini-pro", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest failed: %v", err)
	}

	rawBody, _ := io.ReadAll(req.Body)
	var parsed map[string]interface{}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	contents, ok := parsed["contents"].([]interface{})
	if !ok || len(contents) != 2 {
		t.Fatalf("expected 2 contents, got %d", len(contents))
	}

	userContent, ok := contents[0].(map[string]interface{})
	if !ok {
		t.Fatal("first content is not a map")
	}
	parts, ok := userContent["parts"].([]interface{})
	if !ok || len(parts) != 1 {
		t.Fatalf("expected 1 part in user content, got %d", len(parts))
	}
	partMap, ok := parts[0].(map[string]interface{})
	if !ok {
		t.Fatal("part is not a map")
	}
	if partMap["text"] != "Hello\nWorld" {
		t.Errorf("user text = %v, want %q", partMap["text"], "Hello\nWorld")
	}
}

func TestGeminiAdapter_BuildUpstreamRequest_SystemAsDeveloper(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := &config.ConfigGroup{
		APIURL:  "https://generativelanguage.googleapis.com",
		ModelID: "gemini-pro",
		APIKey:  "test-key",
	}

	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "developer", "content": "You are helpful."},
			{"role": "user", "content": "hi"}
		]
	}`)

	req, err := adapter.BuildUpstreamRequest(context.Background(), group, "gemini-pro", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest failed: %v", err)
	}

	rawBody, _ := io.ReadAll(req.Body)
	var parsed map[string]interface{}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	// developer message should also be converted to systemInstruction
	if _, hasSystem := parsed["systemInstruction"]; !hasSystem {
		t.Error("expected systemInstruction for developer role message")
	}

	// contents should NOT include the developer message
	contents, ok := parsed["contents"].([]interface{})
	if !ok || len(contents) != 1 {
		t.Fatalf("expected 1 content (user only), got %d", len(contents))
	}
	if contents[0].(map[string]interface{})["role"] != "user" {
		t.Error("expected user role in contents")
	}
}

func TestGeminiAdapter_BuildUpstreamRequest_NoMessages(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := &config.ConfigGroup{
		APIURL:  "https://generativelanguage.googleapis.com",
		ModelID: "gemini-pro",
		APIKey:  "test-key",
	}

	body := []byte(`{"model": "gpt-4"}`)

	req, err := adapter.BuildUpstreamRequest(context.Background(), group, "gemini-pro", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest failed: %v", err)
	}

	rawBody, _ := io.ReadAll(req.Body)
	var parsed map[string]interface{}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	// When messages key is absent, buildGeminiContents returns nil.
	// The requestBody always includes "contents" key (even if nil).
	contents, _ := parsed["contents"].([]interface{})
	if contents != nil {
		t.Errorf("expected nil contents, got %v", contents)
	}

	// No system instruction since no messages
	if _, hasSystem := parsed["systemInstruction"]; hasSystem {
		t.Error("unexpected systemInstruction with no messages")
	}
}

func TestGeminiAdapter_BuildUpstreamRequest_Stream(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := &config.ConfigGroup{
		APIURL:  "https://generativelanguage.googleapis.com",
		ModelID: "gemini-pro",
		APIKey:  "test-key",
	}

	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`)

	req, err := adapter.BuildUpstreamRequest(context.Background(), group, "gemini-pro", body, true)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest failed: %v", err)
	}

	url := req.URL.String()
	if !strings.Contains(url, ":streamGenerateContent") {
		t.Errorf("URL should contain :streamGenerateContent, got %q", url)
	}
	if !strings.Contains(url, "alt=sse") {
		t.Errorf("URL should contain alt=sse, got %q", url)
	}
}

func TestGeminiAdapter_BuildUpstreamRequest_NoGenerationConfig(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := &config.ConfigGroup{
		APIURL:  "https://generativelanguage.googleapis.com",
		ModelID: "gemini-pro",
		APIKey:  "test-key",
	}

	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`)

	req, err := adapter.BuildUpstreamRequest(context.Background(), group, "gemini-pro", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest failed: %v", err)
	}

	rawBody, _ := io.ReadAll(req.Body)
	var parsed map[string]interface{}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	// Without temperature/top_p/max_tokens, generationConfig should be absent
	if _, hasCfg := parsed["generationConfig"]; hasCfg {
		t.Error("expected no generationConfig when no config params in input")
	}
}

func TestGeminiAdapter_MapResponseToOpenAI_FinishReason(t *testing.T) {
	adapter := &GeminiAdapter{}

	// SAFETY finish reason
	input := []byte(`{
		"candidates": [{
			"content": {"modelVersion": "gemini-2.0-flash", "parts": [{"text": "no"}]},
			"finishReason": "SAFETY",
			"index": 0
		}],
		"usageMetadata": {"promptTokenCount": 5, "candidatesTokenCount": 2, "totalTokenCount": 7}
	}`)
	out, err := adapter.MapResponseToOpenAI(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]interface{}
	json.Unmarshal(out, &result)
	choices := result["choices"].([]interface{})
	choice := choices[0].(map[string]interface{})
	if choice["finish_reason"] != "content_filter" {
		t.Errorf("finish_reason = %v, want content_filter", choice["finish_reason"])
	}

	// MAX_TOKENS
	input2 := []byte(`{
		"candidates": [{
			"content": {"modelVersion": "gemini-2.0-flash", "parts": [{"text": "long"}]},
			"finishReason": "MAX_TOKENS",
			"index": 0
		}],
		"usageMetadata": {"promptTokenCount": 5, "candidatesTokenCount": 10, "totalTokenCount": 15}
	}`)
	out2, _ := adapter.MapResponseToOpenAI(input2)
	var result2 map[string]interface{}
	json.Unmarshal(out2, &result2)
	choices2 := result2["choices"].([]interface{})
	choice2 := choices2[0].(map[string]interface{})
	if choice2["finish_reason"] != "length" {
		t.Errorf("finish_reason = %v, want length", choice2["finish_reason"])
	}
}

func TestGeminiAdapter_MapStreamToOpenAI_NoFinishReason(t *testing.T) {
	adapter := &GeminiAdapter{}

	// Stream with content but no STOP finish reason — should send a final chunk with stop
	geminiSSE := `data: {"candidates":[{"content":{"parts":[{"text":"Hello"}]},"index":0}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":10,"totalTokenCount":15}}

`

	var buf bytes.Buffer
	err := adapter.MapStreamToOpenAI(strings.NewReader(geminiSSE), &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[DONE]") {
		t.Fatal("expected [DONE] marker when content received but no explicit STOP")
	}
}

func TestGeminiAdapter_MapStreamToOpenAI_BadJSON(t *testing.T) {
	adapter := &GeminiAdapter{}

	// Invalid JSON line should be skipped, not panic
	geminiSSE := `data: not json at all

data: [DONE]

`

	var buf bytes.Buffer
	err := adapter.MapStreamToOpenAI(strings.NewReader(geminiSSE), &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// No content chunks since the JSON is bad, but the function should complete
	if strings.Contains(output, "data: [DONE]") {
		t.Error("unexpected [DONE] after bad JSON input")
	}
}

func TestGeminiAdapter_MapResponseToOpenAI_EmptyParts(t *testing.T) {
	adapter := &GeminiAdapter{}

	input := []byte(`{
		"candidates": [{
			"content": {"parts": []},
			"finishReason": "STOP",
			"index": 0
		}],
		"usageMetadata": {"promptTokenCount": 5, "candidatesTokenCount": 0, "totalTokenCount": 5}
	}`)

	out, err := adapter.MapResponseToOpenAI(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(out, &result)
	choices := result["choices"].([]interface{})
	choice := choices[0].(map[string]interface{})
	if choice["finish_reason"] != "stop" {
		t.Errorf("finish_reason = %v, want stop", choice["finish_reason"])
	}
	// content should be empty string since parts are empty
	if choice["message"].(map[string]interface{})["content"] != "" {
		t.Error("expected empty content for empty parts")
	}
}

func TestGeminiAdapter_MapResponseToOpenAI_ModelVersionFallback(t *testing.T) {
	adapter := &GeminiAdapter{}

	input := []byte(`{
		"candidates": [{
			"content": {"parts": [{"text": "hi"}]},
			"finishReason": "STOP",
			"index": 0
		}]
	}`)

	out, err := adapter.MapResponseToOpenAI(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(out, &result)
	// modelVersion missing → should default to "gemini"
	if result["model"] != "gemini" {
		t.Errorf("model = %v, want gemini", result["model"])
	}
}

func TestGeminiAdapter_BuildUpstreamRequest_OnlySystemMessage(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := &config.ConfigGroup{
		APIURL:  "https://generativelanguage.googleapis.com",
		ModelID: "gemini-pro",
		APIKey:  "test-key",
	}

	body := []byte(`{"model":"gpt-4","messages":[{"role":"system","content":"only system"}]}`)

	req, err := adapter.BuildUpstreamRequest(context.Background(), group, "gemini-pro", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest failed: %v", err)
	}

	rawBody, _ := io.ReadAll(req.Body)
	var parsed map[string]interface{}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	// Only system message → contents is nil (buildGeminiContents returns nil when
	// all messages are system/developer role since they're mapped to systemInstruction)
	if contents := parsed["contents"]; contents != nil {
		t.Errorf("expected nil contents when only system message, got %v", contents)
	}

	// systemInstruction should be present
	if _, hasSystem := parsed["systemInstruction"]; !hasSystem {
		t.Error("expected systemInstruction for system-only message")
	}
}

func TestGeminiAdapter_BuildUpstreamRequest_WithCustomHeaders(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := &config.ConfigGroup{
		APIURL:  "https://generativelanguage.googleapis.com",
		ModelID: "gemini-pro",
		APIKey:  "test-key",
		Headers: map[string]string{"X-Custom-Header": "custom-val"},
	}

	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`)

	req, err := adapter.BuildUpstreamRequest(context.Background(), group, "gemini-pro", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest failed: %v", err)
	}

	if got := req.Header.Get("X-Custom-Header"); got != "custom-val" {
		t.Errorf("X-Custom-Header = %q, want %q", got, "custom-val")
	}
}

func TestGeminiAdapter_BuildUpstreamRequest_WithModelIDRoute(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := &config.ConfigGroup{
		APIURL:  "https://generativelanguage.googleapis.com",
		ModelID: "gemini-2.0-flash",
		APIKey:  "test-key",
	}

	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`)

	req, err := adapter.BuildUpstreamRequest(context.Background(), group, "gemini-2.0-flash", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest failed: %v", err)
	}

	url := req.URL.String()
	if !strings.Contains(url, "gemini-2.0-flash") {
		t.Errorf("URL should contain model id, got %q", url)
	}
	if !strings.Contains(url, "v1beta/models/") {
		t.Errorf("URL should contain v1beta/models/, got %q", url)
	}
}

func TestGeminiAdapter_SetAuthHeaders_NoKey(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := &config.ConfigGroup{
		APIURL: "https://example.com",
		APIKey: "",
	}

	req, err := http.NewRequest("GET", "https://example.com/v1beta/models/test", nil)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	adapter.SetAuthHeaders(req, group)

	q := req.URL.Query()
	if q.Get("key") != "" {
		t.Errorf("query key should be empty when no API key, got %q", q.Get("key"))
	}
}

func TestGeminiAdapter_GetUpstreamURL_CustomAPIURL(t *testing.T) {
	adapter := &GeminiAdapter{}
	group := &config.ConfigGroup{
		APIURL:  "https://custom.api.com",
		ModelID: "custom-model",
		APIKey:  "key",
	}

	got := adapter.GetUpstreamURL(group, false)
	expected := "https://custom.api.com/v1beta/models/custom-model:generateContent"
	if got != expected {
		t.Errorf("URL = %q, want %q", got, expected)
	}
}

