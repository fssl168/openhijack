package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"openhijack/internal/config"
	"openhijack/internal/errors"
	"openhijack/internal/proxy/provider"
)

type UpstreamTransport struct {
	client    *http.Client
	group     *config.ConfigGroup
	config    *config.Config
	adapter   provider.ProviderAdapter
	promptSt  *provider.SystemPromptStore
	debugMode bool
	logger    *slog.Logger
}

func NewUpstreamTransport(cfg *config.Config, debugMode bool, disableSSLStrict bool, logger *slog.Logger) *UpstreamTransport {
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

	adapter, err := provider.GetAdapter(group.Provider)
	if err != nil {
		// Fallback: return a transport without an adapter (will error on forward)
		logger.Error("failed to get provider adapter", "provider", group.Provider, "error", err)
	}

	return &UpstreamTransport{
		client:    client,
		group:     group,
		config:    cfg,
		adapter:   adapter,
		promptSt:  provider.NewSystemPromptStore(),
		debugMode: debugMode,
		logger:    logger,
	}
}

func (t *UpstreamTransport) ForwardChatCompletions(ctx context.Context, requestID string, body []byte, isStream bool) (*http.Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	targetModel := t.config.TargetModelID()

	// Apply system prompt overrides for OpenAI-compatible providers
	adjustedBody, err := provider.ApplySystemPromptOverrides(body, t.promptSt)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternalError, "failed to apply system prompt overrides")
	}

	// Build the upstream request using the provider adapter
	req, err := t.adapter.BuildUpstreamRequest(ctx, t.group, targetModel, adjustedBody, isStream)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrNetworkConnectionFailed, "failed to build upstream request")
	}

	if t.debugMode {
		bodyBytes, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		reqData, _ := json.MarshalIndent(bodyBytes, "", "  ")
		t.logger.Debug("upstream request body", "requestID", requestID, "body", string(reqData))
	}

	upstreamURL := t.adapter.GetUpstreamURL(t.group, isStream)
	t.logger.Info("forwarding to upstream", "requestID", requestID, "url", upstreamURL, "model", targetModel, "stream", isStream)

	t.logger.Debug("request headers", "requestID", requestID, "headers", req.Header)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrNetworkConnectionFailed, "upstream request failed")
	}
	return resp, nil
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

func StreamNonStreamAsSSE(respBody []byte, w http.ResponseWriter, logger *slog.Logger) {
	var respData map[string]interface{}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		logger.Error("failed to parse non-stream response", "error", err)
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
