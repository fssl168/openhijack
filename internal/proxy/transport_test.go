package proxy

import (
	"context"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"

	"openhijack/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testTransportConfig() *config.Config {
	return &config.Config{
		MappedModelID: "mapped-model",
		ConfigGroups: []config.ConfigGroup{
			{
				Name:     "test",
				Provider: config.ProviderOpenAIChatCompletion,
				APIURL:   "https://example.com",
			},
		},
	}
}

func TestNewUpstreamTransportDisablesTLSVerificationWhenRequested(t *testing.T) {
	transport := NewUpstreamTransport(testTransportConfig(), false, true, log.New(io.Discard, "", 0))

	httpTransport, ok := transport.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T", transport.client.Transport)
	}
	if httpTransport.TLSClientConfig == nil || !httpTransport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected TLS verification to be disabled")
	}
}

func TestForwardChatCompletionsUsesRequestContext(t *testing.T) {
	transport := NewUpstreamTransport(testTransportConfig(), false, false, log.New(io.Discard, "", 0))

	type contextKey string
	const key contextKey = "request-id"
	const value = "ctx-value"

	transport.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Context().Value(key); got != value {
			t.Fatalf("context value = %v, want %q", got, value)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if !strings.Contains(string(body), `"model":"mapped-model"`) {
			t.Fatalf("expected mapped model in body: %s", string(body))
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		}, nil
	})

	ctx := context.WithValue(context.Background(), key, value)
	resp, err := transport.ForwardChatCompletions(ctx, "req-1", []byte(`{"model":"client-model","messages":[{"role":"user","content":"hi"}]}`), false)
	if err != nil {
		t.Fatalf("forward chat completions: %v", err)
	}
	defer resp.Body.Close()
}
