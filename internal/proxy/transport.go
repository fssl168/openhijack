package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"openhijack/internal/config"
)

type UpstreamTransport struct {
	client    *http.Client
	group     *config.ConfigGroup
	config    *config.Config
	promptSt  *SystemPromptStore
	debugMode bool
	logger    *log.Logger
}

func NewUpstreamTransport(cfg *config.Config, debugMode bool, disableSSLStrict bool, logger *log.Logger) *UpstreamTransport {
	group := cfg.CurrentGroup()

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if disableSSLStrict {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   0,
	}

	return &UpstreamTransport{
		client:    client,
		group:     group,
		config:    cfg,
		promptSt:  NewSystemPromptStore(),
		debugMode: debugMode,
		logger:    logger,
	}
}

func (t *UpstreamTransport) buildUpstreamURL() string {
	return t.group.FullAPIURL("chat/completions")
}

func (t *UpstreamTransport) ForwardChatCompletions(ctx context.Context, requestID string, body []byte, isStream bool) (*http.Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var reqData map[string]interface{}
	if err := json.Unmarshal(body, &reqData); err != nil {
		return nil, fmt.Errorf("解析请求 JSON 失败: %w", err)
	}

	targetModel := t.config.TargetModelID()
	reqData["model"] = targetModel

	if t.debugMode {
		reqJSON, _ := json.MarshalIndent(reqData, "", "  ")
		t.logger.Printf("[%s] 上游请求体 (调试):\n%s", requestID, string(reqJSON))
	}

	t.applySystemPromptOverrides(requestID, reqData)

	if isStream {
		reqData["stream"] = true
	}

	modifiedBody, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	upstreamURL := t.buildUpstreamURL()
	t.logger.Printf("[%s] 转发到上游: %s (model=%s, stream=%v)", requestID, upstreamURL, targetModel, isStream)

	req, err := http.NewRequestWithContext(ctx, "POST", upstreamURL, bytes.NewReader(modifiedBody))
	if err != nil {
		return nil, fmt.Errorf("创建上游请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if t.group.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.group.APIKey)
	}

	if isStream {
		req.Header.Set("Accept", "text/event-stream")
	}

	return t.client.Do(req)
}

func (t *UpstreamTransport) applySystemPromptOverrides(requestID string, reqData map[string]interface{}) {
	messages, ok := reqData["messages"].([]interface{})
	if !ok {
		return
	}

	var entries []PromptEntry
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
		h := t.promptSt.ComputeHash(text)
		indexedHashes[i] = h
		entries = append(entries, PromptEntry{Hash: h, Text: text})
	}

	if len(entries) == 0 {
		return
	}

	added, overrides := t.promptSt.CaptureAndGetOverrides(entries)
	for _, h := range added {
		t.logger.Printf("[%s] 📝 收录系统提示词 hash=%s", requestID, h[:12])
	}

	if len(overrides) == 0 {
		return
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
			t.logger.Printf("[%s] 🧹 清空系统提示词 hash=%s", requestID, h[:12])
			continue
		}
		msgMap, _ := msg.(map[string]interface{})
		replaced := make(map[string]interface{})
		for k, v := range msgMap {
			replaced[k] = v
		}
		replaced["content"] = override
		newMessages = append(newMessages, replaced)
		t.logger.Printf("[%s] ✏️ 应用系统提示词覆盖 hash=%s", requestID, h[:12])
	}

	if changed {
		reqData["messages"] = newMessages
	}
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

func StreamSSE(resp *http.Response, w http.ResponseWriter, done chan struct{}) {
	defer resp.Body.Close()
	defer close(done)

	flusher, canFlush := w.(http.Flusher)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()

		if _, err := w.Write(line); err != nil {
			return
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return
		}

		if len(line) == 0 {
			if canFlush {
				flusher.Flush()
			}
			continue
		}

		if canFlush {
			flusher.Flush()
		}

		if bytes.Equal(line, []byte("data: [DONE]")) {
			return
		}
	}
}

func StreamNonStreamAsSSE(respBody []byte, w http.ResponseWriter, logger *log.Logger) {
	var respData map[string]interface{}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		logger.Printf("解析非流式响应失败: %v", err)
		w.Write(respBody)
		return
	}

	modelName, _ := respData["model"].(string)

	id, _ := respData["id"].(string)
	if id == "" {
		id = fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	}

	choices, _ := respData["choices"].([]interface{})
	if len(choices) == 0 {
		w.Write(respBody)
		return
	}

	flusher, canFlush := w.(http.Flusher)

	for i, choice := range choices {
		choiceMap, ok := choice.(map[string]interface{})
		if !ok {
			continue
		}

		finishReason := "stop"
		if fr, ok := choiceMap["finish_reason"].(string); ok && fr != "" {
			finishReason = fr
		}

		msg, _ := choiceMap["message"].(map[string]interface{})
		content, _ := msg["content"].(string)

		contentChunk := map[string]interface{}{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   modelName,
			"choices": []interface{}{
				map[string]interface{}{
					"index": i,
					"delta": map[string]interface{}{
						"role":    "assistant",
						"content": content,
					},
					"finish_reason": nil,
				},
			},
		}
		chunkJSON, _ := json.Marshal(contentChunk)
		w.Write([]byte("data: "))
		w.Write(chunkJSON)
		w.Write([]byte("\n\n"))

		finishChunk := map[string]interface{}{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   modelName,
			"choices": []interface{}{
				map[string]interface{}{
					"index":         i,
					"delta":         map[string]interface{}{},
					"finish_reason": finishReason,
				},
			},
		}
		finishJSON, _ := json.Marshal(finishChunk)
		w.Write([]byte("data: "))
		w.Write(finishJSON)
		w.Write([]byte("\n\n"))

		if canFlush {
			flusher.Flush()
		}
	}

	w.Write([]byte("data: [DONE]\n\n"))
	if canFlush {
		flusher.Flush()
	}
}

func ReadBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
