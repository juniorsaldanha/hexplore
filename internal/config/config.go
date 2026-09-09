// Package config loads ~/.config/hexplore/config.toml (docs/PLAN.md §9).
// The API key never lives in this file — see keyring.go.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type ProviderConfig struct {
	// Ordered fallback chain (docs/PLAN.md §2). All speak the Esplora REST
	// surface, so failover is transparent for core data.
	Hosts     []string `toml:"hosts"`
	Network   string   `toml:"network"` // mainnet | testnet | signet
	Timeout   string   `toml:"timeout"` // parsed with time.ParseDuration
	Failover  bool     `toml:"failover"`
	WebSocket bool     `toml:"websocket"`
}

type PriceConfig struct {
	Provider   string   `toml:"provider"` // coingecko | none (only coingecko is implemented so far)
	Currency   string   `toml:"currency"`
	Currencies []string `toml:"currencies"`
}

type DisplayConfig struct {
	Theme          string `toml:"theme"`
	Units          string `toml:"units"`
	Blocks         int    `toml:"blocks"`
	AssumedTxVSize int    `toml:"assumed_tx_vsize"`
	Compact        bool   `toml:"compact"`
	GraphStyle     string `toml:"graph_style"` // braille | block | tty — cycled or set with :graph
}

type RefreshConfig struct {
	Tip     string `toml:"tip"`
	Mempool string `toml:"mempool"`
	Fees    string `toml:"fees"`
	Price   string `toml:"price"`
}

type CacheConfig struct {
	Path    string `toml:"path"`
	MaxSize string `toml:"max_size"`
}

type Config struct {
	Provider ProviderConfig `toml:"provider"`
	Price    PriceConfig    `toml:"price"`
	Display  DisplayConfig  `toml:"display"`
	Refresh  RefreshConfig  `toml:"refresh"`
	Cache    CacheConfig    `toml:"cache"`
}

func Default() Config {
	return Config{
		Provider: ProviderConfig{
			Hosts: []string{
				"https://mempool.space/api",
				"https://mempool.emzy.de/api",
				"https://blockstream.info/api",
			},
			Network:   "mainnet",
			Timeout:   "10s",
			Failover:  true,
			WebSocket: true,
		},
		Price: PriceConfig{
			Provider:   "coingecko",
			Currency:   "USD",
			Currencies: []string{"USD", "BRL"},
		},
		Display: DisplayConfig{
			Theme:          "nord",
			Units:          "btc",
			Blocks:         10,
			AssumedTxVSize: 140,
			Compact:        false,
			GraphStyle:     "braille",
		},
		Refresh: RefreshConfig{
			Tip:     "20s",
			Mempool: "30s",
			Fees:    "60s",
			Price:   "120s",
		},
		Cache: CacheConfig{
			Path:    "~/.cache/hexplore",
			MaxSize: "256MB",
		},
	}
}

// DefaultPath is ~/.config/hexplore/config.toml (or the platform equivalent
// via os.UserConfigDir).
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "hexplore", "config.toml")
}

// Load starts from Default() and overlays path if it exists. A missing file
// is not an error — first run has none yet.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("config: read %s: %w", path, err)
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("config: create dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	return nil
}

// Duration parses a refresh/timeout string, falling back to def on error
// rather than failing startup over a typo'd config value.
func Duration(s string, def time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}

// ExpandPath resolves a leading "~" against the user's home directory —
// config.toml paths like cache.path are written that way for readability.
func ExpandPath(p string) string {
	if p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return p
	}
	if after, ok := strings.CutPrefix(p, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, after)
		}
	}
	return p
}
