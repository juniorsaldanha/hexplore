package app

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		input string
		want  dispatchKind
	}{
		{"912304", dispatchHeight},
		{"0", dispatchHeight},
		{"12345678", dispatchNone}, // 8 digits, too long for a height
		{"000000000000000000010538edbfd2d5b809a33dd83f284aeea41c6d0d96968a", dispatchHash},
		{"21a71ec52cee51aa32db9cb7c1d2a3f016ce9a182403a7d1892761c397c87c4c", dispatchHash},
		{"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", dispatchAddress},
		{"bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq", dispatchAddress},
		{"xpub6BgBgsespWvERF3LHQu6CnqdvfEvtMcQjYrcRzx53QJjSxarj2afYWcLteoGVky7D3UKDP9QyrLprQ3VCECoY49yfdDEHGCtMMj92pReUsQ", dispatchWallet},
		{"zpub6rFR7y4Q2AijBEqTUquhVz398htDFrtymD9xYYfG1m4wAcvPhXNfE3EfH1r1ADqtfSdVCToUG868RvUUkgDKf31mGDtKsAYz2oz2AGutZYs", dispatchWallet},
		{"not-a-real-input", dispatchNone},
	}
	for _, c := range cases {
		if got := classify(c.input); got != c.want {
			t.Errorf("classify(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}
