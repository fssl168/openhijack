package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"openhijack/internal/config"
)

// helper: build a minimal ConfigGroup for testing.
func makeGroup(opts ...func(*config.ConfigGroup)) config.ConfigGroup {
	g := config.ConfigGroup{
		Provider: config.ProviderOpenAIChatCompletion,
		APIURL:   "https://api.openai.com",
	}
	for _, o := range opts {
		o(&g)
	}
	return g
}

func TestOpenAIAdapter_GetUpstreamURL(t *testing.T) {
	a := &OpenAIAdapter{}
	group := makeGroup()
	url := a.GetUpstreamURL(&group, false)
	if got := url; got != "https://api.openai.com/chat/completions" {
		t.Fatalf("GetUpstreamURL = %q, want %q", got, "https://api.openai.com/chat/completions")
	}
	if !bytes.Contains([]byte(url), []byte("/chat/completions")) {
		t.Fatalf("URL %q does not contain /chat/completions", url)
	}
}

func TestOpenAIAdapter_GetUpstreamURL_WithMiddleRoute(t *testing.T) {
	a := &OpenAIAdapter{}
	group := makeGroup(func(g *config.ConfigGroup) {
		g.MiddleRoute = "/v1"
	})
	url := a.GetUpstreamURL(&group, false)
	if got := url; got != "https://api.openai.com/v1/chat/completions" {
		t.Fatalf("GetUpstreamURL = %q, want %q", got, "https://api.openai.com/v1/chat/completions")
	}
	if !bytes.Contains([]byte(url), []byte("/v1/chat/completions")) {
		t.Fatalf("URL %q does not contain /v1/chat/completions", url)
	}
}

func TestOpenAIAdapter_SetAuthHeaders_WithKey(t *testing.T) {
	a := &OpenAIAdapter{}
	req := &http.Request{Header: make(http.Header)}
	group := makeGroup(func(g *config.ConfigGroup) {
		g.APIKey = "sk-xxx"
	})
	a.SetAuthHeaders(req, &group)
	if got := req.Header.Get("Authorization"); got != "Bearer sk-xxx" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer sk-xxx")
	}
}

func TestOpenAIAdapter_SetAuthHeaders_NoKey(t *testing.T) {
	a := &OpenAIAdapter{}
	req := &http.Request{Header: make(http.Header)}
	group := makeGroup(func(g *config.ConfigGroup) {
		g.APIKey = ""
	})
	a.SetAuthHeaders(req, &group)
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q, want empty", got)
	}
}

func TestOpenAIAdapter_BuildUpstreamRequest_Normal(t *testing.T) {
	a := &OpenAIAdapter{}
	body := []byte(`{"model": "gpt-3.5-turbo", "messages": [{"role": "user", "content": "hi"}]}`)
	group := makeGroup(func(g *config.ConfigGroup) {
		g.APIKey = "sk-test"
	})
	ctx := context.Background()
	req, err := a.BuildUpstreamRequest(ctx, &group, "gpt-4", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest error: %v", err)
	}
	// model should be replaced
	var data map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if got := data["model"]; got != "gpt-4" {
		t.Fatalf("model = %v, want %q", got, "gpt-4")
	}
	// Content-Type
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json")
	}
	// Authorization
	if got := req.Header.Get("Authorization"); got != "Bearer sk-test" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer sk-test")
	}
}

func TestOpenAIAdapter_BuildUpstreamRequest_Stream(t *testing.T) {
	a := &OpenAIAdapter{}
	body := []byte(`{"model": "gpt-3.5-turbo", "messages": []}`)
	group := makeGroup(func(g *config.ConfigGroup) {
		g.APIKey = "sk-test"
	})
	ctx := context.Background()
	req, err := a.BuildUpstreamRequest(ctx, &group, "gpt-4", body, true)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest error: %v", err)
	}
	// stream flag — read body from req.Body
	var data map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if got := data["stream"]; got != true {
		t.Fatalf("stream = %v, want true", got)
	}
	// Accept header
	if got := req.Header.Get("Accept"); got != "text/event-stream" {
		t.Fatalf("Accept = %q, want %q", got, "text/event-stream")
	}
}

func TestOpenAIAdapter_BuildUpstreamRequest_NilCtx(t *testing.T) {
	a := &OpenAIAdapter{}
	body := []byte(`{"model": "gpt-3.5-turbo", "messages": []}`)
	group := makeGroup()
	// nil ctx should not panic
	req, err := a.BuildUpstreamRequest(nil, &group, "gpt-4", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest with nil ctx error: %v", err)
	}
	if req == nil {
		t.Fatal("BuildUpstreamRequest with nil ctx returned nil request")
	}
}

func TestOpenAIAdapter_BuildUpstreamRequest_InvalidJSON(t *testing.T) {
	a := &OpenAIAdapter{}
	body := []byte("not json")
	group := makeGroup()
	ctx := context.Background()
	_, err := a.BuildUpstreamRequest(ctx, &group, "gpt-4", body, false)
	if err == nil {
		t.Fatal("BuildUpstreamRequest expected error for invalid JSON, got nil")
	}
}

func TestOpenAIAdapter_BuildUpstreamRequest_CustomHeaders(t *testing.T) {
	a := &OpenAIAdapter{}
	body := []byte(`{"model": "gpt-3.5-turbo", "messages": []}`)
	group := makeGroup(func(g *config.ConfigGroup) {
		g.APIKey = "sk-test"
		g.Headers = map[string]string{"X-Custom": "val"}
	})
	ctx := context.Background()
	req, err := a.BuildUpstreamRequest(ctx, &group, "gpt-4", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest error: %v", err)
	}
	if got := req.Header.Get("X-Custom"); got != "val" {
		t.Fatalf("X-Custom = %q, want %q", got, "val")
	}
}

func TestOpenAIAdapter_ApplySystemPromptOverrides_ArrayContent(t *testing.T) {
	store := NewSystemPromptStore()
	// Register an override for the system prompt hash
	original := "You are a helpful assistant."
	h := store.ComputeHash(original)
	store.SetOverride(h, "OVERRIDDEN: You are a helpful assistant.")

	body := []byte(fmt.Sprintf(`{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": ["%s"]},
			{"role": "user", "content": "hi"}
		]
	}`, original))

	out, err := ApplySystemPromptOverrides(body, store)
	if err != nil {
		t.Fatalf("ApplySystemPromptOverrides error: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to parse output body: %v", err)
	}

	messages, ok := data["messages"].([]interface{})
	if !ok || len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	sysMsg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatal("system message is not a map")
	}
	// The override replaces content with the override text as a string
	if sysMsg["content"] != "OVERRIDDEN: You are a helpful assistant." {
		t.Errorf("content = %v, want override text", sysMsg["content"])
	}
}

func TestOpenAIAdapter_ApplySystemPromptOverrides_ObjectArrayContent(t *testing.T) {
	store := NewSystemPromptStore()
	h := store.ComputeHash("Hello\nWorld")
	store.SetOverride(h, "OVERRIDDEN: Hello\nWorld")

	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": [{"text": "Hello"}, {"text": "World"}]},
			{"role": "user", "content": "hi"}
		]
	}`)

	out, err := ApplySystemPromptOverrides(body, store)
	if err != nil {
		t.Fatalf("ApplySystemPromptOverrides error: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to parse output body: %v", err)
	}

	messages, ok := data["messages"].([]interface{})
	if !ok || len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	sysMsg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatal("system message is not a map")
	}
	if sysMsg["content"] != "OVERRIDDEN: Hello\nWorld" {
		t.Errorf("content = %v, want override text", sysMsg["content"])
	}
}

func TestOpenAIAdapter_ApplySystemPromptOverrides_NonStandardContent(t *testing.T) {
	store := NewSystemPromptStore()
	// No override registered — body should pass through unchanged

	// content is a number (non-standard)
	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": 42},
			{"role": "user", "content": "hi"}
		]
	}`)

	out, err := ApplySystemPromptOverrides(body, store)
	if err != nil {
		t.Fatalf("ApplySystemPromptOverrides error: %v", err)
	}
	// Should be identical to input (non-standard content → extractSystemPromptText returns "")
	if string(out) != string(body) {
		t.Errorf("expected body unchanged, got %s", string(out))
	}

	// content is nil (non-standard)
	bodyNil := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": null},
			{"role": "user", "content": "hi"}
		]
	}`)

	outNil, err := ApplySystemPromptOverrides(bodyNil, store)
	if err != nil {
		t.Fatalf("ApplySystemPromptOverrides error: %v", err)
	}
	if string(outNil) != string(bodyNil) {
		t.Errorf("expected body with null content unchanged, got %s", string(outNil))
	}
}

func TestOpenAIAdapter_ApplySystemPromptOverrides_NoOverride(t *testing.T) {
	store := NewSystemPromptStore()
	// Store is empty — no overrides registered

	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": "some prompt"},
			{"role": "user", "content": "hi"}
		]
	}`)

	out, err := ApplySystemPromptOverrides(body, store)
	if err != nil {
		t.Fatalf("ApplySystemPromptOverrides error: %v", err)
	}
	// Without any overrides, body should pass through unchanged
	if string(out) != string(body) {
		t.Errorf("expected body unchanged, got %s", string(out))
	}
}

func TestOpenAIAdapter_ApplySystemPromptOverrides_OverrideEmpty(t *testing.T) {
	store := NewSystemPromptStore()
	h := store.ComputeHash("some prompt")
	// Set override to empty string — should remove the system message
	store.SetOverride(h, "")

	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": "some prompt"},
			{"role": "user", "content": "hi"}
		]
	}`)

	out, err := ApplySystemPromptOverrides(body, store)
	if err != nil {
		t.Fatalf("ApplySystemPromptOverrides error: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to parse output body: %v", err)
	}

	messages, ok := data["messages"].([]interface{})
	if !ok {
		t.Fatalf("expected messages array")
	}

	// System message should be removed (empty override → skip adding to newMessages)
	var roles []string
	for _, m := range messages {
		msgMap, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		if role, ok := msgMap["role"].(string); ok {
			roles = append(roles, role)
		}
	}
	for _, r := range roles {
		if r == "system" {
			t.Error("system message should have been removed by empty override")
		}
	}
}

func TestOpenAIAdapter_ApplySystemPromptOverrides_DynamicCapture(t *testing.T) {
	store := NewSystemPromptStore()
	// Don't pre-register overrides — capture happens inside ApplySystemPromptOverrides
	// This tests the happy path where capture + no override = body unchanged

	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": "dynamic prompt"},
			{"role": "user", "content": "hi"}
		]
	}`)

	out, err := ApplySystemPromptOverrides(body, store)
	if err != nil {
		t.Fatalf("ApplySystemPromptOverrides error: %v", err)
	}

	// Body should be unchanged since no override was set
	if string(out) != string(body) {
		t.Errorf("expected body unchanged, got %s", string(out))
	}
}

func TestOpenAIAdapter_ApplySystemPromptOverrides_ConverterContent(t *testing.T) {
	store := NewSystemPromptStore()
	// extractSystemPromptText trims whitespace from each part before joining
	// "User: hello" -> "User: hello"
	// "Assistant: hi\n" -> "Assistant: hi" (trimmed)
	// joined: "User: hello\nAssistant: hi"
	h := store.ComputeHash("User: hello\nAssistant: hi")
	store.SetOverride(h, "OVERRIDDEN: User: hello\nAssistant: hi")

	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": [
				{"type": "text", "text": "User: hello"},
				{"type": "text", "text": "Assistant: hi\n"}
			]},
			{"role": "user", "content": "more"}
		]
	}`)

	out, err := ApplySystemPromptOverrides(body, store)
	if err != nil {
		t.Fatalf("ApplySystemPromptOverrides error: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to parse output body: %v", err)
	}

	messages, ok := data["messages"].([]interface{})
	if !ok || len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	sysMsg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatal("system message is not a map")
	}
	if sysMsg["content"] != "OVERRIDDEN: User: hello\nAssistant: hi" {
		t.Errorf("content = %v, want override text", sysMsg["content"])
	}
}

func TestOpenAIAdapter_BuildUpstreamRequest_DevRole(t *testing.T) {
	a := &OpenAIAdapter{}
	body := []byte(`{"model": "gpt-4", "messages": [{"role": "developer", "content": "dev prompt"}, {"role": "user", "content": "hi"}]}`)
	group := makeGroup(func(g *config.ConfigGroup) {
		g.APIKey = "sk-test"
	})
	ctx := context.Background()
	req, err := a.BuildUpstreamRequest(ctx, &group, "gpt-4", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest error: %v", err)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	messages, ok := data["messages"].([]interface{})
	if !ok || len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	// Verify developer role is preserved
	devMsg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatal("first message is not a map")
	}
	if devMsg["role"] != "developer" {
		t.Errorf("role = %v, want 'developer'", devMsg["role"])
	}
}

func TestOpenAIAdapter_BuildUpstreamRequest_EmptyBody(t *testing.T) {
	a := &OpenAIAdapter{}
	body := []byte(`{}`)
	group := makeGroup(func(g *config.ConfigGroup) {
		g.APIKey = "sk-test"
	})
	ctx := context.Background()
	req, err := a.BuildUpstreamRequest(ctx, &group, "gpt-4", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest error: %v", err)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	// Model should be set even when body had no model
	if got := data["model"]; got != "gpt-4" {
		t.Fatalf("model = %v, want %q", got, "gpt-4")
	}
}

func TestOpenAIAdapter_BuildUpstreamRequest_MessagesNull(t *testing.T) {
	a := &OpenAIAdapter{}
	body := []byte(`{"model": "gpt-4", "messages": null}`)
	group := makeGroup()
	ctx := context.Background()
	req, err := a.BuildUpstreamRequest(ctx, &group, "gpt-4", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest error: %v", err)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	if data["model"] != "gpt-4" {
		t.Errorf("model = %v, want %q", data["model"], "gpt-4")
	}
}

func TestOpenAIAdapter_BuildUpstreamRequest_ModelOverride(t *testing.T) {
	a := &OpenAIAdapter{}
	body := []byte(`{"model": "gpt-3.5-turbo", "messages": []}`)
	group := makeGroup()
	ctx := context.Background()
	req, err := a.BuildUpstreamRequest(ctx, &group, "gpt-4-turbo", body, false)
	if err != nil {
		t.Fatalf("BuildUpstreamRequest error: %v", err)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	if got := data["model"]; got != "gpt-4-turbo" {
		t.Fatalf("model = %v, want %q", got, "gpt-4-turbo")
	}
}

func TestMapBodyToModel_InvalidJSON(t *testing.T) {
	_, err := MapBodyToModel([]byte("not json"), "gpt-4")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestMapBodyToModel_Success(t *testing.T) {
	body := []byte(`{"model": "gpt-3.5-turbo", "messages": []}`)
	out, err := MapBodyToModel(body, "gpt-4")
	if err != nil {
		t.Fatalf("MapBodyToModel error: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if data["model"] != "gpt-4" {
		t.Errorf("model = %v, want %q", data["model"], "gpt-4")
	}
}

func TestSystemPromptStore_GetOverride(t *testing.T) {
	store := NewSystemPromptStore()

	text, ok := store.GetOverride("nonexistent")
	if ok {
		t.Errorf("expected ok=false for nonexistent hash, got ok=true, text=%q", text)
	}

	store.SetOverride("myhash", "mytext")
	text, ok = store.GetOverride("myhash")
	if !ok {
		t.Fatal("expected ok=true for existing hash")
	}
	if text != "mytext" {
		t.Errorf("text = %q, want %q", text, "mytext")
	}
}

func TestSystemPromptStore_SetOverride_Empty(t *testing.T) {
	store := NewSystemPromptStore()
	store.SetOverride("h1", "text1")
	store.SetOverride("h1", "")

	text, ok := store.GetOverride("h1")
	if !ok {
		t.Fatal("expected ok=true for override set to empty string")
	}
	if text != "" {
		t.Errorf("text = %q, want empty string", text)
	}
}
