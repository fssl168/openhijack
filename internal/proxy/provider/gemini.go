package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"bufio"

	"openhijack/internal/config"
	"openhijack/internal/errors"
)

func init() {
	Register(config.ProviderGemini, func() ProviderAdapter { return &GeminiAdapter{} })
}

// GeminiAdapter implements ProviderAdapter for Google Gemini API.
type GeminiAdapter struct{}

// GetUpstreamURL returns the Gemini generateContent endpoint URL.
func (a *GeminiAdapter) GetUpstreamURL(group *config.ConfigGroup, isStream bool) string {
	base := group.FullAPIURL("v1beta/models")
	suffix := ":generateContent"
	if isStream {
		suffix = ":streamGenerateContent"
	}
	url := base + "/" + group.ModelID + suffix
	if isStream {
		url += "?alt=sse"
	}
	return url
}

// SetAuthHeaders sets API key as a query parameter.
func (a *GeminiAdapter) SetAuthHeaders(req *http.Request, group *config.ConfigGroup) {
	q := req.URL.Query()
	if group.APIKey != "" {
		q.Set("key", group.APIKey)
	}
	req.URL.RawQuery = q.Encode()
}

// BuildUpstreamRequest constructs an HTTP request for the Gemini API.
func (a *GeminiAdapter) BuildUpstreamRequest(ctx context.Context, group *config.ConfigGroup, targetModel string, body []byte, isStream bool) (*http.Request, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var reqData map[string]interface{}
	if err := json.Unmarshal(body, &reqData); err != nil {
		return nil, errors.Wrap(err, errors.ErrInvalidFormatValue, "failed to parse request JSON")
	}

	messages := reqData["messages"]
	systemText := extractSystemFromMessages(messages)

	// Build Gemini contents from OpenAI messages (filtering out system messages)
	contents := buildGeminiContents(messages)

	requestBody := map[string]interface{}{
		"contents": contents,
	}

	if systemText != "" {
		requestBody["systemInstruction"] = map[string]interface{}{
			"parts": []interface{}{
				map[string]interface{}{"text": systemText},
			},
		}
	}

	// Pass through generation config
	if v, ok := reqData["temperature"]; ok {
		requestBody["generationConfig"] = mergeGenerationConfig(requestBody["generationConfig"], map[string]interface{}{"temperature": v})
	}
	if v, ok := reqData["top_p"]; ok {
		requestBody["generationConfig"] = mergeGenerationConfig(requestBody["generationConfig"], map[string]interface{}{"topP": v})
	}
	if v, ok := reqData["max_tokens"]; ok {
		requestBody["generationConfig"] = mergeGenerationConfig(requestBody["generationConfig"], map[string]interface{}{"maxOutputTokens": v})
	}

	modifiedBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternalError, "failed to serialize request")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.GetUpstreamURL(group, isStream), bytes.NewReader(modifiedBody))
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrNetworkConnectionFailed, "failed to create upstream request")
	}

	req.Header.Set("Content-Type", "application/json")
	a.SetAuthHeaders(req, group)

	for key, value := range group.Headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

// buildGeminiContents converts OpenAI messages array to Gemini contents format.
func buildGeminiContents(messages interface{}) []interface{} {
	msgList, ok := messages.([]interface{})
	if !ok {
		return nil
	}

	var contents []interface{}
	for _, msg := range msgList {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msgMap["role"].(string)

		// Skip system/developer messages — handled separately
		if role == "system" || role == "developer" {
			continue
		}

		// Map OpenAI roles to Gemini roles
		geminiRole := "user"
		if role == "assistant" {
			geminiRole = "model"
		}

		contentText := extractMessageText(msgMap["content"])
		if contentText == "" {
			continue
		}

		contents = append(contents, map[string]interface{}{
			"role":    geminiRole,
			"parts":   []interface{}{map[string]interface{}{"text": contentText}},
		})
	}

	return contents
}

// extractMessageText extracts the text content from an OpenAI message content field.
func extractMessageText(content interface{}) string {
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

// mergeGenerationConfig merges a new map into an existing generationConfig.
func mergeGenerationConfig(existing, update interface{}) interface{} {
	if existing == nil {
		return update
	}
	cfg, ok := existing.(map[string]interface{})
	if !ok {
		return update
	}
	for k, v := range update.(map[string]interface{}) {
		cfg[k] = v
	}
	return cfg
}

// MapResponseToOpenAI converts a Gemini non-stream response to OpenAI ChatCompletion format.
func (a *GeminiAdapter) MapResponseToOpenAI(body []byte) ([]byte, error) {
	var geminiResp map[string]interface{}
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return body, nil
	}

	// Extract candidates
	candidates, ok := geminiResp["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return body, nil
	}

	candidate, ok := candidates[0].(map[string]interface{})
	if !ok {
		return body, nil
	}

	// Extract content
	content, _ := candidate["content"].(map[string]interface{})
	parts, _ := content["parts"].([]interface{})
	text := ""
	for _, part := range parts {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			continue
		}
		if t, _ := partMap["text"].(string); t != "" {
			text += t
		}
	}

	// Map finishReason
	finishReason := "stop"
	if finishRaw, ok := candidate["finishReason"].(string); ok {
		switch finishRaw {
		case "STOP":
			finishReason = "stop"
		case "MAX_TOKENS":
			finishReason = "length"
		case "SAFETY":
			finishReason = "content_filter"
		default:
			finishReason = "stop"
		}
	}

	// Extract model
	model, _ := content["modelVersion"].(string)
	if model == "" {
		model = "gemini"
	}

	// Extract usage
	usageRaw, _ := geminiResp["usageMetadata"].(map[string]interface{})
	promptTokens := int64(0)
	if v, ok := usageRaw["promptTokenCount"].(float64); ok {
		promptTokens = int64(v)
	}
	completionTokens := int64(0)
	if v, ok := usageRaw["candidatesTokenCount"].(float64); ok {
		completionTokens = int64(v)
	}

	id, _ := geminiResp["name"].(string)
	if id == "" {
		id = "chatcmpl-gemini"
	}

	openAIResp := map[string]interface{}{
		"id":      id,
		"object":  "chat.completion",
		"created": 0,
		"model":   model,
		"choices": []interface{}{
			map[string]interface{}{
				"index":         0,
				"message":       map[string]interface{}{"role": "assistant", "content": text},
				"finish_reason": finishReason,
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":   promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":    promptTokens + completionTokens,
		},
	}

	return json.Marshal(openAIResp)
}

// MapStreamToOpenAI converts Gemini SSE stream to OpenAI chat.completion.chunk format.
func (a *GeminiAdapter) MapStreamToOpenAI(src io.Reader, dst io.Writer) error {
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	completionID := "chatcmpl-gemini"
	first := true
	text := ""

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		dataLine := strings.TrimPrefix(line, "data: ")

		var evt map[string]interface{}
		if err := json.Unmarshal([]byte(dataLine), &evt); err != nil {
			continue
		}

		candidates, ok := evt["candidates"].([]interface{})
		if !ok || len(candidates) == 0 {
			continue
		}

		candidate, ok := candidates[0].(map[string]interface{})
		if !ok {
			continue
		}

		content, _ := candidate["content"].(map[string]interface{})
		parts, _ := content["parts"].([]interface{})

		for _, part := range parts {
			partMap, ok := part.(map[string]interface{})
			if !ok {
				continue
			}
			if chunkText, _ := partMap["text"].(string); chunkText != "" {
				text += chunkText
			}
		}

		if first {
			first = false
		} else {
			// Non-first chunks carry accumulated text
			if text == "" {
				continue
			}
		}

		finishReason := "stop"
		var finishRaw string
		if fr, ok := candidate["finishReason"].(string); ok {
			finishRaw = fr
			switch fr {
			case "STOP":
				finishReason = "stop"
			case "MAX_TOKENS":
				finishReason = "length"
			default:
				finishReason = "stop"
			}
		}

		chunk := map[string]interface{}{
			"id":      completionID,
			"object":  "chat.completion.chunk",
			"created": 0,
			"model":   "gemini",
			"choices": []interface{}{
				map[string]interface{}{
					"index": 0,
					"delta": map[string]interface{}{
						"role":    "assistant",
						"content": text,
					},
					"finish_reason": nil,
				},
			},
		}
		chunkJSON, _ := json.Marshal(chunk)
		fmt.Fprintf(dst, "data: %s\n\n", chunkJSON)

		// Check if this is the final chunk
		if finishRaw == "STOP" || finishRaw == "" {
			finishChunk := map[string]interface{}{
				"id":      completionID,
				"object":  "chat.completion.chunk",
				"created": 0,
				"model":   "gemini",
				"choices": []interface{}{
					map[string]interface{}{
						"index":         0,
						"delta":         map[string]interface{}{},
						"finish_reason": finishReason,
					},
				},
			}
			finishJSON, _ := json.Marshal(finishChunk)
			fmt.Fprintf(dst, "data: %s\n\n", finishJSON)
			fmt.Fprintf(dst, "data: [DONE]\n\n")
		}
	}

	// If no finish_reason was seen but we got content, send a final chunk
	if !first && text != "" {
		finishChunk := map[string]interface{}{
			"id":      completionID,
			"object":  "chat.completion.chunk",
			"created": 0,
			"model":   "gemini",
			"choices": []interface{}{
				map[string]interface{}{
					"index":         0,
					"delta":         map[string]interface{}{},
					"finish_reason": "stop",
				},
			},
		}
		finishJSON, _ := json.Marshal(finishChunk)
		fmt.Fprintf(dst, "data: %s\n\n", finishJSON)
		fmt.Fprintf(dst, "data: [DONE]\n\n")
	}

	return scanner.Err()
}
