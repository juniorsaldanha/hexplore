package enrich

import "testing"

func TestClassifyAddress(t *testing.T) {
	cases := []struct {
		addr string
		want string
	}{
		{"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", "P2PKH"},
		{"3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy", "P2SH"},
		{"bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4", "P2WPKH"},                                 // BIP-173 test vector, 42 chars
		{"bc1qrp33g0q5c5txsp9arysrx4k6zdkfs4nce4xj0gdcccefvpysxf3qccfmv3", "P2WSH"},              // BIP-350 test vector, 62 chars
		{"bc1p5d7rjq7g6rdk2yhzks9smlaqtedr4dekq08ge8ztwac72sfr9rusxg3297", "P2TR (Taproot)"},     // shape example: bc1p prefix
		{"n1eCiJ4Ycz9YXfoBoYzMg5nD8PkgAvGKAe", "P2PKH"},                                          // testnet legacy
		{"2N2JD6wb56AfK4tfmM6PwdVmoYk2dCKf4Br", "P2SH"},                                          // testnet script
		{"tb1qw508d6qejxtdg4y5r3zarvary0c5xw7kxpjzsx", "P2WPKH"},                                 // testnet segwit
		{"VJL6PhpCPuYPhAmYqW4gyqNyTVQqfLtVtxxdBGqDXTpFC9wsX2G4pJqPzPbRPqPjhSyUUJyPGkYAyErQ", ""}, // Liquid-shaped, unrecognised
		{"not an address", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := ClassifyAddress(c.addr); got != c.want {
			t.Errorf("ClassifyAddress(%q) = %q, want %q", c.addr, got, c.want)
		}
	}
}
