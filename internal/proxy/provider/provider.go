package provider

import (
	"context"
	"fmt"
	"net/http"

	"openhijack/internal/config"
)

// ProviderAdapter defines the interface for upstream API adapters.
// Each provider (OpenAI, Anthropic, Gemini) implements this to handle
// request building, URL construction, and auth header setting.
type ProviderAdapter interface {
	// BuildUpstreamRequest constructs the HTTP request to send to the upstream API.
	BuildUpstreamRequest(ctx context.Context, group *config.ConfigGroup, targetModel string, body []byte, isStream bool) (*http.Request, error)

	// GetUpstreamURL returns the full upstream URL for this provider.
	GetUpstreamURL(group *config.ConfigGroup, isStream bool) string

	// SetAuthHeaders sets provider-specific auth headers on the request.
	SetAuthHeaders(req *http.Request, group *config.ConfigGroup)
}

// Registry maps provider names to factory functions.
var registry = map[string]func() ProviderAdapter{}

// Register adds a provider adapter factory to the registry.
func Register(name string, factory func() ProviderAdapter) {
	registry[name] = factory
}

// GetAdapter returns a new instance of the adapter for the given provider name.
func GetAdapter(providerName string) (ProviderAdapter, error) {
	if factory, ok := registry[providerName]; ok {
		return factory(), nil
	}
	return nil, fmt.Errorf("unsupported provider: %s", providerName)
}
