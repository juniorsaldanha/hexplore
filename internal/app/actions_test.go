package app

import (
	"testing"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

func TestCurrentTargetDashboard(t *testing.T) {
	m, _ := newTestModel()
	kind, id, ok := m.currentTarget()
	if !ok || kind != "block" || id != "hash0" {
		t.Fatalf("currentTarget() = (%q, %q, %v), want (block, hash0, true)", kind, id, ok)
	}
}

func TestCurrentTargetNoneWhenNoBlocks(t *testing.T) {
	m, _ := newTestModel()
	m.blocks = nil
	_, _, ok := m.currentTarget()
	if ok {
		t.Fatal("expected no target with no blocks and no screen pushed")
	}
}

func TestCurrentTargetInsideTxScreen(t *testing.T) {
	m, _ := newTestModel()
	m.push(screen{kind: screenTx, tx: domain.Tx{TxID: "abc"}})
	kind, id, ok := m.currentTarget()
	if !ok || kind != "tx" || id != "abc" {
		t.Fatalf("currentTarget() = (%q, %q, %v), want (tx, abc, true)", kind, id, ok)
	}
}

func TestYankWithNothingSelected(t *testing.T) {
	m, _ := newTestModel()
	m.blocks = nil
	m.yank()
	if m.statusMsg != "nothing selected to yank" {
		t.Fatalf("statusMsg = %q", m.statusMsg)
	}
}

func TestCycleUnit(t *testing.T) {
	m, _ := newTestModel()
	if m.units.String() != "btc" {
		t.Fatalf("initial unit = %v, want btc", m.units)
	}
	m.cycleUnit()
	if m.units.String() != "sats" {
		t.Fatalf("after 1 cycle = %v, want sats", m.units)
	}
	m.cycleUnit()
	if m.units.String() != "fiat" {
		t.Fatalf("after 2 cycles = %v, want fiat", m.units)
	}
	m.cycleUnit()
	if m.units.String() != "btc" {
		t.Fatalf("after 3 cycles = %v, want btc (wrap around)", m.units)
	}
}

func TestCycleTheme(t *testing.T) {
	m, _ := newTestModel()
	m.theme = theme.Nord
	m.cycleTheme()
	if m.theme.Name != "dracula" {
		t.Fatalf("theme after cycle = %q, want dracula", m.theme.Name)
	}
}

func TestCycleCurrency(t *testing.T) {
	m, _ := newTestModel()
	m.cfg.Currencies = []string{"USD", "BRL"}
	if got := m.currentCurrency(); got != "USD" {
		t.Fatalf("currentCurrency() = %q, want USD", got)
	}
	m.cycleCurrency()
	if got := m.currentCurrency(); got != "BRL" {
		t.Fatalf("after cycle currentCurrency() = %q, want BRL", got)
	}
	m.cycleCurrency()
	if got := m.currentCurrency(); got != "USD" {
		t.Fatalf("after 2nd cycle currentCurrency() = %q, want USD (wrap)", got)
	}
}

func TestCycleCurrencySingleConfigured(t *testing.T) {
	m, _ := newTestModel()
	m.cfg.Currencies = nil // falls back to cfg.Currency
	m.cycleCurrency()
	if m.statusMsg != "only one currency configured" {
		t.Fatalf("statusMsg = %q", m.statusMsg)
	}
}
