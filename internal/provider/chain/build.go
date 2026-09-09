package chain

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/juniorsaldanha/hexplore/internal/provider"
	"github.com/juniorsaldanha/hexplore/internal/provider/esplora"
	"github.com/juniorsaldanha/hexplore/internal/provider/mempoolspace"
)

// NewFromHosts builds a Chain from config base URLs, adjusting each for
// network and picking the mempool.space adapter (native /v1 extras) for
// hosts known to run that software, esplora otherwise.
//
// ponytail: hostname-based heuristic — a self-hosted mempool.space instance
// not named "mempool*" won't be detected as one and loses the /v1 extras.
// Add an explicit per-host type to config if that turns out to matter.
func NewFromHosts(hosts []string, network string) (*Chain, error) {
	if len(hosts) == 0 {
		return nil, fmt.Errorf("chain: no hosts configured")
	}
	providers := make([]provider.ChainProvider, 0, len(hosts))
	for _, h := range hosts {
		u, err := networkize(h, network)
		if err != nil {
			return nil, err
		}
		if isMempoolSpaceHost(u) {
			providers = append(providers, mempoolspace.New(u))
		} else {
			providers = append(providers, esplora.New(u))
		}
	}
	return New(providers...), nil
}

func isMempoolSpaceHost(baseURL string) bool {
	u, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(u.Host), "mempool")
}

// networkize inserts the network path segment for the known default hosts —
// mainnet needs nothing, testnet/signet do (verified against the live APIs).
// A self-hosted or unrecognized host is left unchanged: point `hosts`
// straight at your own testnet/signet instance instead.
func networkize(baseURL, network string) (string, error) {
	if network == "" || network == "mainnet" {
		return baseURL, nil
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("chain: bad host %q: %w", baseURL, err)
	}
	host := strings.ToLower(u.Host)
	switch {
	case strings.Contains(host, "mempool.space"), strings.Contains(host, "mempool.emzy.de"):
		seg := network
		if network == "testnet" {
			seg = "testnet4" // mempool.space's current testnet is testnet4, not testnet3
		}
		u.Path = "/" + seg + u.Path
	case strings.Contains(host, "blockstream.info"):
		if network == "signet" {
			return "", fmt.Errorf("chain: blockstream.info has no signet API; use mempool.space or a self-hosted host")
		}
		u.Path = "/" + network + u.Path
	}
	return u.String(), nil
}
