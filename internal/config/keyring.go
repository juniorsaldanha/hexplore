package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/zalando/go-keyring"
)

const keyringService = "hexplore"

// APIKey looks up a provider's API key: HEXPLORE_API_KEY first (the
// documented, less-secure headless/CI fallback), then the system keyring.
// An empty result with a nil error means "no key configured" — most
// providers, mempool.space included, need none at all.
func APIKey(providerName string) (string, error) {
	if v := os.Getenv("HEXPLORE_API_KEY"); v != "" {
		return v, nil
	}
	v, err := keyring.Get(keyringService, providerName+"-api-key")
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("config: keyring unavailable, set HEXPLORE_API_KEY instead: %w", err)
	}
	return v, nil
}

// SetAPIKey stores a provider's API key in the system keyring — what the
// config screen calls. Never writes to the TOML file.
func SetAPIKey(providerName, key string) error {
	if err := keyring.Set(keyringService, providerName+"-api-key", key); err != nil {
		return fmt.Errorf("config: keyring unavailable, set HEXPLORE_API_KEY instead: %w", err)
	}
	return nil
}
