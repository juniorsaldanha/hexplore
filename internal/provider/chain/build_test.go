package chain

import "testing"

func TestNetworkize(t *testing.T) {
	cases := []struct {
		baseURL, network, want string
		wantErr                bool
	}{
		{"https://mempool.space/api", "mainnet", "https://mempool.space/api", false},
		{"https://mempool.space/api", "testnet", "https://mempool.space/testnet4/api", false},
		{"https://mempool.space/api", "signet", "https://mempool.space/signet/api", false},
		{"https://mempool.emzy.de/api", "testnet", "https://mempool.emzy.de/testnet4/api", false},
		{"https://blockstream.info/api", "testnet", "https://blockstream.info/testnet/api", false},
		{"https://blockstream.info/api", "signet", "", true},
		{"https://my-self-hosted.example/api", "testnet", "https://my-self-hosted.example/api", false},
	}
	for _, c := range cases {
		got, err := networkize(c.baseURL, c.network)
		if c.wantErr {
			if err == nil {
				t.Errorf("networkize(%q, %q) expected an error", c.baseURL, c.network)
			}
			continue
		}
		if err != nil {
			t.Errorf("networkize(%q, %q): %v", c.baseURL, c.network, err)
			continue
		}
		if got != c.want {
			t.Errorf("networkize(%q, %q) = %q, want %q", c.baseURL, c.network, got, c.want)
		}
	}
}

func TestIsMempoolSpaceHost(t *testing.T) {
	if !isMempoolSpaceHost("https://mempool.space/api") {
		t.Error("mempool.space should be detected")
	}
	if !isMempoolSpaceHost("https://mempool.emzy.de/api") {
		t.Error("mempool.emzy.de should be detected")
	}
	if isMempoolSpaceHost("https://blockstream.info/api") {
		t.Error("blockstream.info should not be detected as mempool.space")
	}
}
