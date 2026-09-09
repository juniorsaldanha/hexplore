package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Provider.Network != "mainnet" || len(cfg.Provider.Hosts) == 0 {
		t.Fatalf("expected defaults, got %+v", cfg)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexplore", "config.toml")
	cfg := Default()
	cfg.Provider.Network = "testnet"
	cfg.Price.Currency = "BRL"
	cfg.Display.Blocks = 5

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Provider.Network != "testnet" || got.Price.Currency != "BRL" || got.Display.Blocks != 5 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir in this environment")
	}
	if got := ExpandPath("~/.cache/hexplore"); got != filepath.Join(home, ".cache/hexplore") {
		t.Errorf("ExpandPath(~/.cache/hexplore) = %q", got)
	}
	if got := ExpandPath("/absolute/path"); got != "/absolute/path" {
		t.Errorf("ExpandPath should leave absolute paths alone, got %q", got)
	}
}

func TestDurationFallsBackOnBadInput(t *testing.T) {
	if got := Duration("not-a-duration", 5*time.Second); got != 5*time.Second {
		t.Errorf("Duration fallback = %v, want 5s", got)
	}
	if got := Duration("30s", time.Second); got != 30*time.Second {
		t.Errorf("Duration parse = %v, want 30s", got)
	}
}
