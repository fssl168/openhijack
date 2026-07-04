package provider

import (
	"encoding/json"
	"testing"
)

func TestRegisterAndGetAdapter(t *testing.T) {
	// Verify built-in adapters are registered
	for _, name := range []string{"openai_chat_completion", "anthropic", "gemini"} {
		adapter, err := GetAdapter(name)
		if err != nil {
			t.Fatalf("GetAdapter(%q) = %v, want nil", name, err)
		}
		if adapter == nil {
			t.Fatalf("GetAdapter(%q) returned nil adapter", name)
		}
	}
}

func TestGetAdapterUnknown(t *testing.T) {
	_, err := GetAdapter("unknown_provider")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}

func TestRegisterOverwrite(t *testing.T) {
	Register("test_provider", func() ProviderAdapter { return &OpenAIAdapter{} })
	adapter, err := GetAdapter("test_provider")
	if err != nil {
		t.Fatalf("GetAdapter(test_provider) = %v", err)
	}
	if _, ok := adapter.(*OpenAIAdapter); !ok {
		t.Fatalf("expected *OpenAIAdapter, got %T", adapter)
	}
}

func TestMapBodyToModel(t *testing.T) {
	original := []byte(`{"model":"client-model","messages":[{"role":"user","content":"hi"}]}`)
	modified, err := MapBodyToModel(original, "mapped-model")
	if err != nil {
		t.Fatalf("MapBodyToModel = %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(modified, &data); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if data["model"] != "mapped-model" {
		t.Fatalf("model = %v, want mapped-model", data["model"])
	}
}

func TestMapBodyToModelInvalidJSON(t *testing.T) {
	_, err := MapBodyToModel([]byte("not json"), "model")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSystemPromptStore_ComputeHash(t *testing.T) {
	store := NewSystemPromptStore()
	h1 := store.ComputeHash("hello")
	h2 := store.ComputeHash("hello")
	h3 := store.ComputeHash("world")

	if h1 != h2 {
		t.Fatal("same input should produce same hash")
	}
	if h1 == h3 {
		t.Fatal("different input should produce different hash")
	}
	if len(h1) != 64 {
		t.Fatalf("hash length = %d, want 64", len(h1))
	}
}

func TestSystemPromptStore_CaptureAndGetOverrides(t *testing.T) {
	store := NewSystemPromptStore()
	entries := []SystemPromptEntry{
		{Hash: "hash1", Text: "system prompt 1"},
		{Hash: "hash2", Text: "system prompt 2"},
	}

	added, _ := store.CaptureAndGetOverrides(entries)
	if len(added) != 2 {
		t.Fatalf("added = %d, want 2", len(added))
	}

	// Set an override
	store.SetOverride("hash1", "override 1")
	_, newOverrides := store.CaptureAndGetOverrides(entries)
	if _, ok := newOverrides["hash1"]; !ok {
		t.Fatal("expected hash1 override to be active")
	}

	// GetOverride
	text, found := store.GetOverride("hash1")
	if !found || text != "override 1" {
		t.Fatalf("GetOverride = %q, want override 1", text)
	}

	// Non-existent override
	_, found = store.GetOverride("nonexistent")
	if found {
		t.Fatal("expected false for nonexistent override")
	}
}

func TestSystemPromptStore_DuplicateCapture(t *testing.T) {
	store := NewSystemPromptStore()
	entries := []SystemPromptEntry{
		{Hash: "hash1", Text: "prompt 1"},
		{Hash: "hash1", Text: "prompt 1 duplicate"},
	}

	added, _ := store.CaptureAndGetOverrides(entries)
	if len(added) != 1 {
		t.Fatalf("added = %d, want 1 (duplicate should be skipped)", len(added))
	}
}

func TestApplySystemPromptOverrides(t *testing.T) {
	store := NewSystemPromptStore()

	// Capture the system prompt
	original := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": "You are a helpful assistant"},
			{"role": "user", "content": "hi"}
		]
	}`)

	// Apply overrides — no override set yet, body should be unchanged
	result, err := ApplySystemPromptOverrides(original, store)
	if err != nil {
		t.Fatalf("ApplySystemPromptOverrides = %v", err)
	}

	// Now set an override
	store.SetOverride(store.ComputeHash("You are a helpful assistant"), "You are a sarcastic assistant")

	result, err = ApplySystemPromptOverrides(original, store)
	if err != nil {
		t.Fatalf("ApplySystemPromptOverrides = %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(result, &data); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	messages := data["messages"].([]interface{})
	systemMsg := messages[0].(map[string]interface{})
	if systemMsg["content"] != "You are a sarcastic assistant" {
		t.Fatalf("system content = %q, want 'You are a sarcastic assistant'", systemMsg["content"])
	}
}

func TestApplySystemPromptOverridesClear(t *testing.T) {
	store := NewSystemPromptStore()
	original := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": "old prompt"},
			{"role": "user", "content": "hi"}
		]
	}`)

	// Clear the system prompt
	store.SetOverride(store.ComputeHash("old prompt"), "")

	result, err := ApplySystemPromptOverrides(original, store)
	if err != nil {
		t.Fatalf("ApplySystemPromptOverrides = %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(result, &data); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	messages := data["messages"].([]interface{})
	// After clearing, the system message should be removed (continue skips it)
	if len(messages) != 1 {
		t.Fatalf("messages length = %d, want 1 (system removed)", len(messages))
	}
	if msg, _ := messages[0].(map[string]interface{})["role"].(string); msg != "user" {
		t.Fatalf("first message role = %q, want user", msg)
	}
}

func TestApplySystemPromptOverridesNoSystem(t *testing.T) {
	store := NewSystemPromptStore()
	original := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "hi"}
		]
	}`)

	result, err := ApplySystemPromptOverrides(original, store)
	if err != nil {
		t.Fatalf("ApplySystemPromptOverrides = %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(result, &data); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	// Body should be unchanged since there are no system messages
	if data["model"] != "gpt-4" {
		t.Fatal("model should be unchanged")
	}
}

func TestApplySystemPromptOverridesDeveloperRole(t *testing.T) {
	store := NewSystemPromptStore()
	original := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "developer", "content": "developer prompt"}
		]
	}`)

	store.SetOverride(store.ComputeHash("developer prompt"), "override developer")
	result, err := ApplySystemPromptOverrides(original, store)
	if err != nil {
		t.Fatalf("ApplySystemPromptOverrides = %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(result, &data); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	messages := data["messages"].([]interface{})
	systemMsg := messages[0].(map[string]interface{})
	if systemMsg["content"] != "override developer" {
		t.Fatalf("developer content = %q, want 'override developer'", systemMsg["content"])
	}
}
