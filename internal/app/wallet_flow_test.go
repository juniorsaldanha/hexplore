package app

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/juniorsaldanha/hexplore/internal/cache"
	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/wallet"
)

func typeCommand(m *Model, cmd string) {
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	for _, r := range cmd {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func TestWalletCommandUnambiguousKeyOpensDirectly(t *testing.T) {
	const zpub = "zpub6rFR7y4Q2AijBEqTUquhVz398htDFrtymD9xYYfG1m4wAcvPhXNfE3EfH1r1ADqtfSdVCToUG868RvUUkgDKf31mGDtKsAYz2oz2AGutZYs"
	m, fc := newTestModel()
	fc.addrs = map[string]domain.Address{} // no activity anywhere; scan should still complete

	typeCommand(m, "wallet "+zpub)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal (no ambiguity for a zpub), got %v", m.mode)
	}
	if len(m.stack) != 1 || m.stack[0].kind != screenWallet {
		t.Fatalf("expected a wallet screen pushed, stack=%v", m.stack)
	}
	drain(t, m, cmd)
	if m.stack[0].wallet.ScriptType != wallet.NativeSegWit.String() {
		t.Errorf("ScriptType = %q, want %q", m.stack[0].wallet.ScriptType, wallet.NativeSegWit.String())
	}
}

func TestWalletCommandAmbiguousKeyPromptsBeforeScanning(t *testing.T) {
	const xpub = "xpub6BgBgsespWvERF3LHQu6CnqdvfEvtMcQjYrcRzx53QJjSxarj2afYWcLteoGVky7D3UKDP9QyrLprQ3VCECoY49yfdDEHGCtMMj92pReUsQ"

	m, fc := newTestModel()
	// Populate every Taproot address the scan could touch once the user
	// picks Taproot below (2 chains x up to walletGapLimit*3, generous).
	key, err := wallet.Parse(xpub)
	if err != nil {
		t.Fatal(err)
	}
	key.ScriptType = wallet.Taproot
	addrs := map[string]domain.Address{}
	for chainIdx := uint32(0); chainIdx < 2; chainIdx++ {
		for index := uint32(0); index < walletGapLimit*3; index++ {
			addr, err := key.Address(chainIdx, index)
			if err != nil {
				t.Fatal(err)
			}
			addrs[addr] = domain.Address{Address: addr}
		}
	}
	fc.addrs = addrs

	typeCommand(m, "wallet "+xpub)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		t.Fatal("expected no command yet — the ambiguity prompt must come first")
	}
	if m.mode != modeWalletScriptType {
		t.Fatalf("expected modeWalletScriptType, got %v", m.mode)
	}
	if len(m.stack) != 0 {
		t.Fatalf("expected no screen pushed before the user resolves the ambiguity, stack=%v", m.stack)
	}
	if !strings.Contains(m.pendingWalletKey, "xpub6BgBgsesp") {
		t.Errorf("pendingWalletKey = %q, want it to hold the xpub", m.pendingWalletKey)
	}

	// Select Taproot (second choice) and confirm.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if walletScriptTypeChoices[m.walletScriptTypeCursor] != wallet.Taproot {
		t.Fatalf("expected Taproot highlighted after one 'j', cursor=%d", m.walletScriptTypeCursor)
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after confirming, got %v", m.mode)
	}
	if len(m.stack) != 1 || m.stack[0].kind != screenWallet {
		t.Fatalf("expected a wallet screen pushed after confirming, stack=%v", m.stack)
	}
	drain(t, m, cmd)
	if m.stack[0].wallet.ScriptType != wallet.Taproot.String() {
		t.Errorf("ScriptType = %q, want %q", m.stack[0].wallet.ScriptType, wallet.Taproot.String())
	}
}

func TestWalletScriptTypePromptEscCancels(t *testing.T) {
	const xpub = "xpub6BgBgsespWvERF3LHQu6CnqdvfEvtMcQjYrcRzx53QJjSxarj2afYWcLteoGVky7D3UKDP9QyrLprQ3VCECoY49yfdDEHGCtMMj92pReUsQ"
	m, _ := newTestModel()

	typeCommand(m, "wallet "+xpub)
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeWalletScriptType {
		t.Fatalf("expected modeWalletScriptType, got %v", m.mode)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after Esc, got %v", m.mode)
	}
	if m.pendingWalletKey != "" {
		t.Errorf("expected pendingWalletKey cleared after Esc, got %q", m.pendingWalletKey)
	}
	if len(m.stack) != 0 {
		t.Errorf("expected no screen pushed, stack=%v", m.stack)
	}
}

func populateAmbiguousXpubAddresses(t *testing.T, fc *fakeChain, st wallet.ScriptType) string {
	t.Helper()
	const xpub = "xpub6BgBgsespWvERF3LHQu6CnqdvfEvtMcQjYrcRzx53QJjSxarj2afYWcLteoGVky7D3UKDP9QyrLprQ3VCECoY49yfdDEHGCtMMj92pReUsQ"
	key, err := wallet.Parse(xpub)
	if err != nil {
		t.Fatal(err)
	}
	key.ScriptType = st
	addrs := map[string]domain.Address{}
	for chainIdx := uint32(0); chainIdx < 2; chainIdx++ {
		for index := uint32(0); index < walletGapLimit*3; index++ {
			addr, err := key.Address(chainIdx, index)
			if err != nil {
				t.Fatal(err)
			}
			addrs[addr] = domain.Address{Address: addr}
		}
	}
	fc.addrs = addrs
	return xpub
}

func TestWalletScriptTypeChoiceIsRememberedForTheSession(t *testing.T) {
	m, fc := newTestModel()
	xpub := populateAmbiguousXpubAddresses(t, fc, wallet.Taproot)

	// First open: ambiguous, must prompt. Resolve to Taproot.
	typeCommand(m, "wallet "+xpub)
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeWalletScriptType {
		t.Fatalf("expected a prompt on first open, mode=%v", m.mode)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // Taproot
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	drain(t, m, cmd)
	m.stack = nil // back out, as if the user pressed Esc back to the dashboard

	// Second open of the exact same key: must NOT prompt again.
	typeCommand(m, "wallet "+xpub)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode == modeWalletScriptType {
		t.Fatal("expected no prompt the second time — the choice should be remembered for the session")
	}
	if len(m.stack) != 1 || m.stack[0].kind != screenWallet {
		t.Fatalf("expected a wallet screen pushed directly, stack=%v", m.stack)
	}
	drain(t, m, cmd)
	if m.stack[0].wallet.ScriptType != wallet.Taproot.String() {
		t.Errorf("ScriptType = %q, want %q", m.stack[0].wallet.ScriptType, wallet.Taproot.String())
	}
}

func TestWalletScriptTypeChoicePersistsToWatchlist(t *testing.T) {
	wl, err := cache.OpenWatchlist(filepath.Join(t.TempDir(), "watchlist.db"))
	if err != nil {
		t.Fatalf("OpenWatchlist: %v", err)
	}
	defer wl.Close()

	m, fc := newTestModel()
	m.cfg.Watchlist = wl
	xpub := populateAmbiguousXpubAddresses(t, fc, wallet.Taproot)

	// Watch it first (no derivation known yet).
	typeCommand(m, "watch "+xpub)
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Resolve the derivation via a direct open.
	typeCommand(m, "wallet "+xpub)
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeWalletScriptType {
		t.Fatalf("expected a prompt, mode=%v", m.mode)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // Taproot
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	wallets, err := wl.ListWallets()
	if err != nil {
		t.Fatalf("ListWallets: %v", err)
	}
	if len(wallets) != 1 || wallets[0].ScriptType != wallet.Taproot.Name() {
		t.Fatalf("expected the persisted entry to carry the resolved ScriptType, got %+v", wallets)
	}

	// A brand-new model (simulating a restart) sharing the same store must
	// not prompt either — the saved choice comes from the store itself.
	m2, fc2 := newTestModel()
	m2.cfg.Watchlist = wl
	populateAmbiguousXpubAddresses(t, fc2, wallet.Taproot)
	typeCommand(m2, "wallet "+xpub)
	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m2.mode == modeWalletScriptType {
		t.Fatal("expected no prompt — the choice was persisted to the shared watchlist store")
	}
	drain(t, m2, cmd)
	if len(m2.stack) != 1 || m2.stack[0].wallet.ScriptType != wallet.Taproot.String() {
		t.Fatalf("expected the restored session to scan as Taproot, got %+v", m2.stack)
	}
}
