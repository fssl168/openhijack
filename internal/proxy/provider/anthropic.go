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
	Register(config.ProviderAnthropic, func() ProviderAdapter { return &AnthropicAdapter{} })
}

const anthropicAPIVersion = "2023-06-01"

// AnthropicAdapter implements ProviderAdapter for Anthropic Messages API.
type AnthropicAdapter struct{}

// GetUpstreamURL returns the Anthropic Messages endpoint URL.
func (a *AnthropicAdapter) GetUpstreamURL(group *config.ConfigGroup, _ bool) string {
	return group.FullAPIURL("v1/messages")
}

// SetAuthHeaders sets Anthropic-specific auth headers.
func (a *AnthropicAdapter) SetAuthHeaders(req *http.Request, group *config.ConfigGroup) {
	if group.APIKey != "" {
		req.Header.Set("x-api-key", group.APIKey)
	}
	req.Header.Set("anthropic-version", anthropicAPIVersion)
}

// BuildUpstreamRequest constructs an HTTP request for the Anthropic Messages API.
func (a *AnthropicAdapter) BuildUpstreamRequest(ctx context.Context, group *config.ConfigGroup, targetModel string, body []byte, isStream bool) (*http.Request, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var reqData map[string]interface{}
	if err := json.Unmarshal(body, &reqData); err != nil {
		return nil, errors.Wrap(err, errors.ErrInvalidFormatValue, "failed to parse request JSON")
	}

	messages := reqData["messages"]
	system := extractSystemFromMessages(messages)

	requestBody := map[string]interface{}{
		"model":  targetModel,
		"messages": messages,
	}

	// Pass through common params
	if v, ok := reqData["max_tokens"]; ok {
		requestBody["max_tokens"] = v
	}
	if v, ok := reqData["temperature"]; ok {
		requestBody["temperature"] = v
	}
	if v, ok := reqData["top_p"]; ok {
		requestBody["top_p"] = v
	}
	if v, ok := reqData["stream"]; ok {
		requestBody["stream"] = v
	}

	if system != "" {
		requestBody["system"] = system
	}

	modifiedBody, err := json.Marshal(requestBody)
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

	for key, value := range group.Headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

// extractSystemFromMessages extracts system prompt text from OpenAI-style messages.
// Anthropic puts system at the top level, so we remove system messages and return their text.
func extractSystemFromMessages(messages interface{}) string {
	msgList, ok := messages.([]interface{})
	if !ok {
		return ""
	}

	var parts []string
	for _, msg := range msgList {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msgMap["role"].(string)
		if role != "system" && role != "developer" {
			continue
		}
		if text := extractSystemPromptText(msgMap["content"]); text != "" {
			parts = append(parts, text)
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

// MapResponseToOpenAI converts an Anthropic non-stream response to OpenAI ChatCompletion format.
func (a *AnthropicAdapter) MapResponseToOpenAI(body []byte) ([]byte, error) {
	var anthResp map[string]interface{}
	if err := json.Unmarshal(body, &anthResp); err != nil {
		return body, nil
	}

	// Extract content blocks
	contentBlocks, _ := anthResp["content"].([]interface{})
	contentText := ""
	for _, block := range contentBlocks {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		if blockType, _ := blockMap["type"].(string); blockType == "text" {
			if text, _ := blockMap["text"].(string); text != "" {
				contentText += text
			}
		}
	}

	// Extract usage
	usage, _ := anthResp["usage"].(map[string]interface{})
	promptTokens := int64(0)
	if v, ok := usage["input_tokens"].(float64); ok {
		promptTokens = int64(v)
	}
	completionTokens := int64(0)
	if v, ok := usage["output_tokens"].(float64); ok {
		completionTokens = int64(v)
	}

	// Map stop_reason to finish_reason
	stopReason := "stop"
	if sr, ok := anthResp["stop_reason"].(string); ok {
		switch sr {
		case "end_turn", "stop_sequence":
			stopReason = "stop"
		case "max_tokens":
			stopReason = "length"
		case "tool_use":
			stopReason = "tool_calls"
		default:
			stopReason = sr
		}
	}

	// Get model id
	model, _ := anthResp["model"].(string)
	if model == "" {
		model = "anthropic"
	}

	// Build OpenAI-like response
	openAIResp := map[string]interface{}{
		"id":      anthResp["id"],
		"object":  "chat.completion",
		"created": float64(0),
		"model":   model,
		"choices": []interface{}{
			map[string]interface{}{
				"index":   0,
				"message": map[string]interface{}{"role": "assistant", "content": contentText},
				"finish_reason": stopReason,
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

// MapStreamToOpenAI converts Anthropic SSE stream events to OpenAI chat.completion.chunk format.
func (a *AnthropicAdapter) MapStreamToOpenAI(src io.Reader, dst io.Writer) error {
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var completionID string
	first := true

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "event: ") {
			continue
		}

		eventType := strings.TrimPrefix(line, "event: ")
		eventType = strings.TrimSpace(eventType)

		// Read the data line
		if !scanner.Scan() {
			break
		}
		dataLine := scanner.Text()
		if !strings.HasPrefix(dataLine, "data: ") {
			continue
		}
		dataLine = strings.TrimPrefix(dataLine, "data: ")

		if eventType == "message_start" {
			var evt map[string]interface{}
			if err := json.Unmarshal([]byte(dataLine), &evt); err == nil {
				if id, ok := evt["id"].(string); ok {
					completionID = id
				}
			}
			continue
		}

		if eventType == "content_block_delta" {
			var evt map[string]interface{}
			if err := json.Unmarshal([]byte(dataLine), &evt); err != nil {
				continue
			}

			delta, ok := evt["delta"].(map[string]interface{})
			if !ok {
				continue
			}
			deltaType, _ := delta["type"].(string)
			if deltaType != "text_delta" {
				continue
			}

			text, _ := delta["text"].(string)
			if text == "" {
				continue
			}

			if first {
				if completionID == "" {
					completionID = "chatcmpl-anthropic"
				}
				first = false
			}

			chunk := map[string]interface{}{
				"id":      completionID,
				"object":  "chat.completion.chunk",
				"created": 0,
				"model":   "anthropic",
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
			continue
		}

		if eventType == "message_delta" {
			var evt map[string]interface{}
			if err := json.Unmarshal([]byte(dataLine), &evt); err != nil {
				continue
			}

			stopReason := "stop"
			if delta, ok := evt["delta"].(map[string]interface{}); ok {
				if sr, _ := delta["stop_reason"].(string); sr != "" {
					switch sr {
					case "end_turn", "stop_sequence":
						stopReason = "stop"
					case "max_tokens":
						stopReason = "length"
					default:
						stopReason = sr
					}
				}
			}

			if completionID == "" {
				completionID = "chatcmpl-anthropic"
			}

			chunk := map[string]interface{}{
				"id":      completionID,
				"object":  "chat.completion.chunk",
				"created": 0,
				"model":   "anthropic",
				"choices": []interface{}{
					map[string]interface{}{
						"index":         0,
						"delta":         map[string]interface{}{},
						"finish_reason": stopReason,
					},
				},
			}
			chunkJSON, _ := json.Marshal(chunk)
			fmt.Fprintf(dst, "data: %s\n\n", chunkJSON)

			fmt.Fprintf(dst, "data: [DONE]\n\n")
		}
	}

	return scanner.Err()
}
