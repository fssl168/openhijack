package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

type ConfigGroup struct {
	Name                   string `toml:"name"`
	Provider               string `toml:"provider"`
	APIURL                 string `toml:"api_url"`
	ModelID                string `toml:"model_id"`
	APIKey                 string `toml:"api_key"`
	MiddleRoute            string `toml:"middle_route"`
	ModelDiscoveryStrategy string `toml:"model_discovery_strategy"`
	PromptCacheEnabled     bool   `toml:"prompt_cache_enabled"`
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
		normalizeConfigGroup(&cfg.ConfigGroups[i])
	}

	return &cfg, nil
}

func normalizeConfigGroup(g *ConfigGroup) {
	if g.Provider == "" {
		g.Provider = "openai_chat_completion"
	}
	g.Provider = normalizeProvider(g.Provider)
	g.MiddleRoute = strings.TrimRight(g.MiddleRoute, "/")
}

func normalizeProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai_chat_completion", "openai":
		return "openai_chat_completion"
	case "openai_response":
		return "openai_response"
	case "anthropic":
		return "anthropic"
	case "gemini":
		return "gemini"
	default:
		return "openai_chat_completion"
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
