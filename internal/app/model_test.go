package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/poll"
	"github.com/juniorsaldanha/hexplore/internal/provider"
)

// fakeChain is a canned provider.ChainProvider for driving Model.Update
// without a network or a real terminal.
type fakeChain struct {
	name         string
	blocks       map[string]domain.Block // by hash
	txs          map[string]domain.Tx
	addrs        map[string]domain.Address
	projectedTxs []domain.ProjectedTx
	projectedErr error
	liveBlockTxs bool // sets Caps().LiveBlockTxs

	// addrTxByCursor scripts AddressTxs as a pure lookup keyed by the
	// lastSeen cursor ("" for the first page) — stateless, so two
	// independent pagination walks over the same address (e.g. the tx
	// list fetch and fetchAddressHistory's own separate walk) see the
	// same deterministic pages rather than racing a shared call counter.
	addrTxByCursor map[string][]domain.Tx
}

var _ provider.ChainProvider = (*fakeChain)(nil)

func (f *fakeChain) Name() string {
	if f.name != "" {
		return f.name
	}
	return "fake"
}
func (f *fakeChain) Caps() domain.Capabilities {
	return domain.Capabilities{BlockFees: true, MiningPool: true, NextBlockExact: true, LiveBlockTxs: f.liveBlockTxs}
}
func (f *fakeChain) TipHeight(context.Context) (int, error) { return 100, nil }
func (f *fakeChain) LatestBlocks(context.Context, int) ([]domain.Block, error) {
	var out []domain.Block
	for _, b := range f.blocks {
		out = append(out, b)
	}
	return out, nil
}
func (f *fakeChain) BlockByHeight(ctx context.Context, height int) (domain.Block, error) {
	for _, b := range f.blocks {
		if b.Height == height {
			return b, nil
		}
	}
	return domain.Block{}, errors.New("not found")
}
func (f *fakeChain) BlockByHash(ctx context.Context, hash string) (domain.Block, error) {
	b, ok := f.blocks[hash]
	if !ok {
		return domain.Block{}, errors.New("not found")
	}
	return b, nil
}
func (f *fakeChain) BlockTxs(ctx context.Context, hash string, start int) ([]domain.Tx, error) {
	var out []domain.Tx
	for _, t := range f.txs {
		out = append(out, t)
	}
	return out, nil
}
func (f *fakeChain) Tx(ctx context.Context, txid string) (domain.Tx, error) {
	t, ok := f.txs[txid]
	if !ok {
		return domain.Tx{}, errors.New("not found")
	}
	return t, nil
}
func (f *fakeChain) Address(ctx context.Context, addr string) (domain.Address, error) {
	a, ok := f.addrs[addr]
	if !ok {
		return domain.Address{}, errors.New("not found")
	}
	return a, nil
}
func (f *fakeChain) AddressTxs(ctx context.Context, addr, lastSeen string) ([]domain.Tx, error) {
	if f.addrTxByCursor == nil {
		return nil, nil
	}
	return f.addrTxByCursor[lastSeen], nil
}
func (f *fakeChain) AddressPrefix(ctx context.Context, prefix string) ([]string, error) {
	return nil, provider.ErrUnsupported
}
func (f *fakeChain) Mempool(context.Context) (domain.MempoolState, error) {
	return domain.MempoolState{}, nil
}
func (f *fakeChain) FeeEstimates(context.Context) (domain.FeeTiers, error) {
	return domain.FeeTiers{}, nil
}
func (f *fakeChain) NextBlocks(context.Context, int) ([]domain.ProjectedBlock, error) {
	return nil, nil
}
func (f *fakeChain) ProjectedBlockTxs(context.Context, int) ([]domain.ProjectedTx, error) {
	return f.projectedTxs, f.projectedErr
}

func newTestModel() (*Model, *fakeChain) {
	fc := &fakeChain{
		blocks: map[string]domain.Block{
			"hash0": {Height: 100, Hash: "hash0"},
			"hash1": {Height: 99, Hash: "hash1"},
		},
		txs: map[string]domain.Tx{
			"tx0": {TxID: "tx0", Fee: 500},
		},
		addrs: map[string]domain.Address{
			"bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq": {Address: "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq", FundedSats: 100},
		},
	}
	m := New(fc, nil, Config{Network: "mainnet", Currency: "USD", NumBlocks: 2, AssumedVSize: 140})
	m.blocks = []domain.Block{fc.blocks["hash0"], fc.blocks["hash1"]}
	m.width, m.height = 120, 40
	return m, fc
}

// drain runs a tea.Cmd synchronously and feeds its Msg back into Update —
// the deterministic stand-in for bubbletea's real event loop in tests.
func drain(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	// tea.Batch's own Cmd returns a tea.BatchMsg ([]tea.Cmd); the real
	// runtime runs each sub-command independently. Only the first is
	// drained here by convention (callers in this codebase always batch
	// the data fetch first) — a self-re-arming tea.Tick is often the
	// second, and actually invoking it blocks for its real duration, which
	// a unit test must never do.
	if batch, ok := msg.(tea.BatchMsg); ok {
		if len(batch) > 0 {
			drain(t, m, batch[0])
		}
		return
	}
	if _, next := m.Update(msg); next != nil {
		drain(t, m, next)
	}
}

func TestDashboardCursorMovement(t *testing.T) {
	m, _ := newTestModel()
	if m.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", m.cursor)
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.cursor != 1 {
		t.Fatalf("cursor after j = %d, want 1", m.cursor)
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // already at bottom
	if m.cursor != 1 {
		t.Fatalf("cursor should clamp at len-1, got %d", m.cursor)
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.cursor != 0 {
		t.Fatalf("cursor after k = %d, want 0", m.cursor)
	}
}

func TestEnterOpensBlockAndEscPops(t *testing.T) {
	m, _ := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.stack) != 1 || m.stack[0].kind != screenBlock {
		t.Fatalf("expected a block screen pushed, got stack=%v", m.stack)
	}
	drain(t, m, cmd)
	if m.stack[0].block.Hash != "hash0" {
		t.Fatalf("expected block hash0 loaded, got %+v", m.stack[0].block)
	}
	if len(m.stack[0].blockTxs) != 1 {
		t.Fatalf("expected 1 tx loaded, got %d", len(m.stack[0].blockTxs))
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if len(m.stack) != 0 {
		t.Fatalf("expected Esc to pop back to dashboard, stack=%v", m.stack)
	}
}

func TestSearchDispatchAddress(t *testing.T) {
	m, _ := newTestModel()
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if m.mode != modeSearch {
		t.Fatalf("expected search mode after /, got %v", m.mode)
	}
	addr := "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq"
	for _, r := range addr {
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeNormal {
		t.Fatalf("expected mode back to normal after search submit, got %v", m.mode)
	}
	if len(m.stack) != 1 || m.stack[0].kind != screenAddress {
		t.Fatalf("expected an address screen pushed, got stack=%v", m.stack)
	}
	drain(t, m, cmd)
	if m.stack[0].address.Address != addr {
		t.Fatalf("expected address loaded, got %+v", m.stack[0].address)
	}
}

func TestSearchRejectsGarbage(t *testing.T) {
	m, _ := newTestModel()
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	for _, r := range "??" {
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeSearch {
		t.Fatalf("expected to stay in search mode on bad input, got %v", m.mode)
	}
	if m.searchErr == "" {
		t.Fatal("expected a searchErr to be set")
	}
	if len(m.stack) != 0 {
		t.Fatalf("expected no screen pushed for garbage input, stack=%v", m.stack)
	}
}

func TestPollMsgUpdatesStateAndKeepsListening(t *testing.T) {
	m, _ := newTestModel()
	_, cmd := m.Update(poll.MempoolMsg{State: domain.MempoolState{Count: 42}})
	if m.mempool.Count != 42 {
		t.Fatalf("mempool count = %d, want 42", m.mempool.Count)
	}
	if cmd == nil {
		t.Fatal("expected Update to keep listening for the next scheduler message")
	}
}

func TestRenderDoesNotPanic(t *testing.T) {
	m, _ := newTestModel()
	if out := m.View(); !strings.Contains(out, "hexplore") {
		t.Fatalf("dashboard view missing header, got:\n%s", out)
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = m.View() // block screen while loading, must not panic
}
