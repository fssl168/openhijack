package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"openhijack/internal/config"
	"openhijack/internal/errors"
)

func init() {
	Register(config.ProviderOpenAIChatCompletion, func() ProviderAdapter { return &OpenAIAdapter{} })
}

// OpenAIAdapter implements ProviderAdapter for OpenAI-compatible Chat Completions APIs.
type OpenAIAdapter struct{}

// GetUpstreamURL returns the Chat Completions endpoint URL.
func (a *OpenAIAdapter) GetUpstreamURL(group *config.ConfigGroup, _ bool) string {
	return group.FullAPIURL("chat/completions")
}

// SetAuthHeaders sets the Bearer auth header on the request.
func (a *OpenAIAdapter) SetAuthHeaders(req *http.Request, group *config.ConfigGroup) {
	if group.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+group.APIKey)
	}
}

// BuildUpstreamRequest constructs an HTTP request for the Chat Completions endpoint.
func (a *OpenAIAdapter) BuildUpstreamRequest(ctx context.Context, group *config.ConfigGroup, targetModel string, body []byte, isStream bool) (*http.Request, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var reqData map[string]interface{}
	if err := json.Unmarshal(body, &reqData); err != nil {
		return nil, errors.Wrap(err, errors.ErrInvalidFormatValue, "failed to parse request JSON")
	}

	reqData["model"] = targetModel

	if isStream {
		reqData["stream"] = true
	}

	modifiedBody, err := json.Marshal(reqData)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternalError, "failed to serialize request")
	}

	url := a.GetUpstreamURL(group, isStream)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(modifiedBody))
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrNetworkConnectionFailed, "failed to create upstream request")
	}

	req.Header.Set("Content-Type", "application/json")
	a.SetAuthHeaders(req, group)
	if isStream {
		req.Header.Set("Accept", "text/event-stream")
	}

	for key, value := range group.Headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

// MapBodyToModel overrides the model field in the request body.
// Returns the modified JSON bytes, or the original body if no changes needed.
func MapBodyToModel(body []byte, targetModel string) ([]byte, error) {
	var reqData map[string]interface{}
	if err := json.Unmarshal(body, &reqData); err != nil {
		return nil, errors.Wrap(err, errors.ErrInvalidFormatValue, "failed to parse request JSON")
	}

	reqData["model"] = targetModel
	return json.Marshal(reqData)
}

// SystemPromptEntry is a prompt text with its hash.
type SystemPromptEntry struct {
	Hash string
	Text string
}

// SystemPromptStore stores captured system prompts and their overrides.
type SystemPromptStore struct {
	prompts   map[string]string
	overrides map[string]string
}

// NewSystemPromptStore creates a new empty store.
func NewSystemPromptStore() *SystemPromptStore {
	return &SystemPromptStore{
		prompts:   make(map[string]string),
		overrides: make(map[string]string),
	}
}

// ComputeHash returns the SHA-256 hex hash of the given text.
func (s *SystemPromptStore) ComputeHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

// CaptureAndGetOverrides returns newly captured hashes and active overrides.
func (s *SystemPromptStore) CaptureAndGetOverrides(entries []SystemPromptEntry) ([]string, map[string]string) {
	var added []string
	for _, e := range entries {
		if _, exists := s.prompts[e.Hash]; !exists {
			s.prompts[e.Hash] = e.Text
			added = append(added, e.Hash)
		}
	}

	active := make(map[string]string)
	for k, v := range s.overrides {
		if _, exists := s.prompts[k]; exists {
			active[k] = v
		}
	}
	return added, active
}

// SetOverride sets an override for the given hash.
func (s *SystemPromptStore) SetOverride(hash, text string) {
	s.overrides[hash] = text
}

// GetOverride returns the override text for the given hash.
func (s *SystemPromptStore) GetOverride(hash string) (string, bool) {
	text, ok := s.overrides[hash]
	return text, ok
}

// ApplySystemPromptOverrides finds system/developer messages in the messages array
// and replaces their content with cached overrides. Returns the modified body.
func ApplySystemPromptOverrides(body []byte, promptStore *SystemPromptStore) ([]byte, error) {
	var reqData map[string]interface{}
	if err := json.Unmarshal(body, &reqData); err != nil {
		return body, nil
	}

	messages, ok := reqData["messages"].([]interface{})
	if !ok {
		return body, nil
	}

	var entries []SystemPromptEntry
	indexedHashes := make(map[int]string)

	for i, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msgMap["role"].(string)
		if role != "system" && role != "developer" {
			continue
		}
		text := extractSystemPromptText(msgMap["content"])
		if text == "" {
			continue
		}
		h := promptStore.ComputeHash(text)
		indexedHashes[i] = h
		entries = append(entries, SystemPromptEntry{Hash: h, Text: text})
	}

	if len(entries) == 0 {
		return body, nil
	}

	_, overrides := promptStore.CaptureAndGetOverrides(entries)
	if len(overrides) == 0 {
		return body, nil
	}

	var newMessages []interface{}
	changed := false
	for i, msg := range messages {
		h, hasHash := indexedHashes[i]
		if !hasHash {
			newMessages = append(newMessages, msg)
			continue
		}
		override, hasOverride := overrides[h]
		if !hasOverride {
			newMessages = append(newMessages, msg)
			continue
		}
		changed = true
		if override == "" {
			continue
		}
		msgMap, _ := msg.(map[string]interface{})
		replaced := make(map[string]interface{})
		for k, v := range msgMap {
			replaced[k] = v
		}
		replaced["content"] = override
		newMessages = append(newMessages, replaced)
	}

	if changed {
		reqData["messages"] = newMessages
	}

	return json.Marshal(reqData)
}

func extractSystemPromptText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return strings.TrimSpace(v)
	case []interface{}:
		var parts []string
		for _, item := range v {
			switch iv := item.(type) {
			case string:
				if t := strings.TrimSpace(iv); t != "" {
					parts = append(parts, t)
				}
			case map[string]interface{}:
				if text, ok := iv["text"].(string); ok {
					if t := strings.TrimSpace(text); t != "" {
						parts = append(parts, t)
					}
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}
