package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

const ProviderOpenAIChatCompletion = "openai_chat_completion"

type ConfigGroup struct {
	Name                   string            `toml:"name"`
	Provider               string            `toml:"provider"`
	APIURL                 string            `toml:"api_url"`
	ModelID                string            `toml:"model_id"`
	APIKey                 string            `toml:"api_key"`
	MiddleRoute            string            `toml:"middle_route"`
	ModelDiscoveryStrategy string            `toml:"model_discovery_strategy"`
	PromptCacheEnabled     bool              `toml:"prompt_cache_enabled"`
	Headers                map[string]string `toml:"headers"`
}

type Config struct {
	MappedModelID       string        `toml:"mapped_model_id"`
	AuthKey             string        `toml:"auth_key"`
	CurrentConfigIndex  int           `toml:"current_config_index"`
	ConfigGroups        []ConfigGroup `toml:"config_groups"`
	PromptCacheBucketID string        `toml:"prompt_cache_bucket_id"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	if len(cfg.ConfigGroups) == 0 {
		return nil, fmt.Errorf("配置文件中没有 config_groups")
	}

	if cfg.CurrentConfigIndex < 0 || cfg.CurrentConfigIndex >= len(cfg.ConfigGroups) {
		cfg.CurrentConfigIndex = 0
	}

	for i := range cfg.ConfigGroups {
		if err := normalizeConfigGroup(&cfg.ConfigGroups[i]); err != nil {
			return nil, fmt.Errorf("校验 config_groups[%d] 失败: %w", i, err)
		}
	}

	return &cfg, nil
}

func normalizeConfigGroup(g *ConfigGroup) error {
	provider, err := normalizeProvider(g.Provider)
	if err != nil {
		return err
	}
	g.Provider = provider
	g.MiddleRoute = strings.TrimRight(g.MiddleRoute, "/")
	return validateConfigGroup(g)
}

func normalizeProvider(provider string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", ProviderOpenAIChatCompletion, "openai":
		return ProviderOpenAIChatCompletion, nil
	case "openai_response":
		return "openai_response", nil
	case "anthropic":
		return "anthropic", nil
	case "gemini":
		return "gemini", nil
	default:
		return "", fmt.Errorf("不支持的 provider: %q", provider)
	}
}

func validateConfigGroup(g *ConfigGroup) error {
	switch g.Provider {
	case ProviderOpenAIChatCompletion:
		return nil
	case "openai_response", "anthropic", "gemini":
		return fmt.Errorf("provider %q 尚未实现，目前仅支持 %q", g.Provider, ProviderOpenAIChatCompletion)
	default:
		return fmt.Errorf("不支持的 provider: %q", g.Provider)
	}
}

func (c *Config) CurrentGroup() *ConfigGroup {
	if c.CurrentConfigIndex >= 0 && c.CurrentConfigIndex < len(c.ConfigGroups) {
		return &c.ConfigGroups[c.CurrentConfigIndex]
	}
	return &c.ConfigGroups[0]
}

func (c *Config) TargetModelID() string {
	g := c.CurrentGroup()
	if g.ModelID != "" {
		return g.ModelID
	}
	return c.MappedModelID
}

func (g *ConfigGroup) TargetAPIBaseURL() string {
	return strings.TrimRight(g.APIURL, "/")
}

func (g *ConfigGroup) FullAPIURL(suffix string) string {
	base := g.TargetAPIBaseURL()
	middle := g.MiddleRoute
	if middle != "" && middle != "/" {
		base = base + middle
	}
	return base + "/" + strings.TrimPrefix(suffix, "/")
}
