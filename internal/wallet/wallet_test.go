package wallet

import "testing"

// TestParseAndAddressAgainstOfficialVectors is the highest-value test in
// this package: real, published BIP84/BIP49 test vectors (from the BIPs'
// own specs, not derived by this code) verify Parse's SLIP-132 version
// handling and Address's per-ScriptType derivation actually produce the
// correct real-world addresses, not just something plausible-looking.
func TestParseAndAddressAgainstOfficialVectors(t *testing.T) {
	t.Run("BIP84 zpub (native segwit, mainnet)", func(t *testing.T) {
		const zpub = "zpub6rFR7y4Q2AijBEqTUquhVz398htDFrtymD9xYYfG1m4wAcvPhXNfE3EfH1r1ADqtfSdVCToUG868RvUUkgDKf31mGDtKsAYz2oz2AGutZYs"
		k, err := Parse(zpub)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if k.ScriptType != NativeSegWit {
			t.Fatalf("ScriptType = %v, want NativeSegWit", k.ScriptType)
		}
		cases := []struct {
			chain, index uint32
			want         string
		}{
			{0, 0, "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu"},
			{0, 1, "bc1qnjg0jd8228aq7egyzacy8cys3knf9xvrerkf9g"},
			{1, 0, "bc1q8c6fshw2dlwun7ekn9qwf37cu2rn755upcp6el"},
		}
		for _, c := range cases {
			got, err := k.Address(c.chain, c.index)
			if err != nil {
				t.Fatalf("Address(%d, %d): %v", c.chain, c.index, err)
			}
			if got != c.want {
				t.Errorf("Address(%d, %d) = %s, want %s", c.chain, c.index, got, c.want)
			}
		}
	})

	t.Run("BIP86 xpub overridden to Taproot", func(t *testing.T) {
		const xpub = "xpub6BgBgsespWvERF3LHQu6CnqdvfEvtMcQjYrcRzx53QJjSxarj2afYWcLteoGVky7D3UKDP9QyrLprQ3VCECoY49yfdDEHGCtMMj92pReUsQ"
		k, err := Parse(xpub)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if k.ScriptType != Legacy {
			t.Fatalf("ScriptType = %v, want Legacy (Parse's default for a plain xpub)", k.ScriptType)
		}
		k.ScriptType = Taproot // what AmbiguousScriptType exists to prompt for

		cases := []struct {
			chain, index uint32
			want         string
		}{
			{0, 0, "bc1p5cyxnuxmeuwuvkwfem96lqzszd02n6xdcjrs20cac6yqjjwudpxqkedrcr"},
			{1, 0, "bc1p3qkhfews2uk44qtvauqyr2ttdsw7svhkl9nkm9s9c3x4ax5h60wqwruhk7"},
		}
		for _, c := range cases {
			got, err := k.Address(c.chain, c.index)
			if err != nil {
				t.Fatalf("Address(%d, %d): %v", c.chain, c.index, err)
			}
			if got != c.want {
				t.Errorf("Address(%d, %d) = %s, want %s", c.chain, c.index, got, c.want)
			}
		}
	})

	t.Run("BIP49 upub (nested segwit, testnet)", func(t *testing.T) {
		const upub = "upub5EFU65HtV5TeiSHmZZm7FUffBGy8UKeqp7vw43jYbvZPpoVsgU93oac7Wk3u6moKegAEWtGNF8DehrnHtv21XXEMYRUocHqguyjknFHYfgY"
		k, err := Parse(upub)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if k.ScriptType != NestedSegWit {
			t.Fatalf("ScriptType = %v, want NestedSegWit", k.ScriptType)
		}
		got, err := k.Address(0, 0)
		if err != nil {
			t.Fatalf("Address(0, 0): %v", err)
		}
		if want := "2Mww8dCYPUpKHofjgcXcBCEGmniw9CoaiD2"; got != want {
			t.Errorf("Address(0, 0) = %s, want %s", got, want)
		}
	})
}

func TestParseRejectsPrivateKeys(t *testing.T) {
	// The zprv counterpart of the BIP84 test vector above.
	const zprv = "zprvAdG4iTXWBoARxkkzNpNh8r6Qag3irQB8PzEMkAFeTRXxHpbF9z4QgEvBRmfvqWvGp42t42nvgGpNgYSJA9iefm1yYNZKEm7z6qUWCroSQnE"
	_, err := Parse(zprv)
	if err != ErrPrivateKey {
		t.Fatalf("Parse(zprv) error = %v, want ErrPrivateKey", err)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	cases := []string{
		"",
		"not a key at all",
		"bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh", // a plain address, not an extended key
		"zpub6rFR7y4Q2AijBEqTUquhVz398htDFrtymD9xYYfG1m4wAcvPhXNfE3EfH1r1ADqtfSdVCToUG868RvUUkgDKf31mGDtKsAYz2oz2AGutZY0", // last char corrupted -> bad checksum
	}
	for _, c := range cases {
		if _, err := Parse(c); err == nil {
			t.Errorf("Parse(%q) succeeded, want an error", c)
		}
	}
}

func TestAmbiguousScriptType(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"xpub6BgBgsespWvERF3LHQu6CnqdvfEvtMcQjYrcRzx53QJjSxarj2afYWcLteoGVky7D3UKDP9QyrLprQ3VCECoY49yfdDEHGCtMMj92pReUsQ", true},  // xpub — legacy or taproot
		{"zpub6rFR7y4Q2AijBEqTUquhVz398htDFrtymD9xYYfG1m4wAcvPhXNfE3EfH1r1ADqtfSdVCToUG868RvUUkgDKf31mGDtKsAYz2oz2AGutZYs", false}, // zpub — native segwit only
		{"upub5EFU65HtV5TeiSHmZZm7FUffBGy8UKeqp7vw43jYbvZPpoVsgU93oac7Wk3u6moKegAEWtGNF8DehrnHtv21XXEMYRUocHqguyjknFHYfgY", false}, // upub — nested segwit only
		{"zprvAdG4iTXWBoARxkkzNpNh8r6Qag3irQB8PzEMkAFeTRXxHpbF9z4QgEvBRmfvqWvGp42t42nvgGpNgYSJA9iefm1yYNZKEm7z6qUWCroSQnE", false}, // private key — not even a valid watch-only input
		{"not a key", false},
	}
	for _, c := range cases {
		if got := AmbiguousScriptType(c.key); got != c.want {
			t.Errorf("AmbiguousScriptType(%q) = %v, want %v", c.key, got, c.want)
		}
	}
}

func TestScriptTypeString(t *testing.T) {
	cases := map[ScriptType]string{
		Legacy:       "legacy (P2PKH)",
		NestedSegWit: "nested SegWit (P2SH-P2WPKH)",
		NativeSegWit: "native SegWit (P2WPKH)",
		Taproot:      "Taproot (P2TR)",
	}
	for st, want := range cases {
		if got := st.String(); got != want {
			t.Errorf("%v.String() = %q, want %q", int(st), got, want)
		}
	}
}

func TestScriptTypeNameRoundTrips(t *testing.T) {
	for _, st := range []ScriptType{Legacy, NestedSegWit, NativeSegWit, Taproot} {
		got, ok := ParseScriptTypeName(st.Name())
		if !ok || got != st {
			t.Errorf("ParseScriptTypeName(%q) = %v, %v; want %v, true", st.Name(), got, ok, st)
		}
	}
	if _, ok := ParseScriptTypeName("garbage"); ok {
		t.Error("ParseScriptTypeName(garbage) should fail")
	}
}
