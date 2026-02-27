package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const DefaultRefresh = 60 * time.Second
const MinRefresh = 30 * time.Second

type Config struct {
	RefreshInterval time.Duration `json:"refresh_interval"`
	PinnedOnTop     bool          `json:"pinned_on_top"`

	// Usage tracking (from current 5h block)
	InputTokensUsed  int     `json:"input_tokens_used"`
	OutputTokensUsed int     `json:"output_tokens_used"`
	BlockTotalTokens int     `json:"block_total_tokens"`
	IndicatorPercent float64 `json:"indicator_percent"`
}

func appDataDir() string {
	if p := os.Getenv("APPDATA"); p != "" {
		return filepath.Join(p, "ClaudeTokenWidget")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude-token-widget")
}

func FilePath() string {
	return filepath.Join(appDataDir(), "config.json")
}

func Load() *Config {
	cfg := &Config{
		RefreshInterval: DefaultRefresh,
	}

	if data, err := os.ReadFile(FilePath()); err == nil {
		json.Unmarshal(data, cfg)
	}

	if cfg.RefreshInterval < MinRefresh {
		cfg.RefreshInterval = DefaultRefresh
	}

	return cfg
}

func (c *Config) Save() error {
	dir := appDataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(FilePath(), data, 0o600)
}
