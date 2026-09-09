package chain

import (
	"context"
	"errors"
	"testing"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/provider"
	"github.com/juniorsaldanha/hexplore/internal/provider/esplora"
)

// stubHost is a minimal provider.ChainProvider whose TipHeight is scripted.
type stubHost struct {
	name    string
	calls   int
	tipErrs []error // one per call; last one repeats once exhausted
	tip     int
}

func (s *stubHost) Name() string              { return s.name }
func (s *stubHost) Caps() domain.Capabilities { return domain.Capabilities{} }
func (s *stubHost) TipHeight(context.Context) (int, error) {
	i := s.calls
	if i >= len(s.tipErrs) {
		i = len(s.tipErrs) - 1
	}
	s.calls++
	if s.tipErrs[i] != nil {
		return 0, s.tipErrs[i]
	}
	return s.tip, nil
}
func (s *stubHost) LatestBlocks(context.Context, int) ([]domain.Block, error) { return nil, nil }
func (s *stubHost) BlockByHeight(context.Context, int) (domain.Block, error) {
	return domain.Block{}, nil
}
func (s *stubHost) BlockByHash(context.Context, string) (domain.Block, error) {
	return domain.Block{}, nil
}
func (s *stubHost) BlockTxs(context.Context, string, int) ([]domain.Tx, error) { return nil, nil }
func (s *stubHost) Tx(context.Context, string) (domain.Tx, error)              { return domain.Tx{}, nil }
func (s *stubHost) Address(context.Context, string) (domain.Address, error) {
	return domain.Address{}, nil
}
func (s *stubHost) AddressTxs(context.Context, string, string) ([]domain.Tx, error) { return nil, nil }
func (s *stubHost) AddressPrefix(context.Context, string) ([]string, error)         { return nil, nil }
func (s *stubHost) Mempool(context.Context) (domain.MempoolState, error) {
	return domain.MempoolState{}, nil
}
func (s *stubHost) FeeEstimates(context.Context) (domain.FeeTiers, error) {
	return domain.FeeTiers{}, nil
}
func (s *stubHost) NextBlocks(context.Context, int) ([]domain.ProjectedBlock, error) { return nil, nil }
func (s *stubHost) ProjectedBlockTxs(context.Context, int) ([]domain.ProjectedTx, error) {
	return nil, nil
}

func TestFailoverOnRetryableError(t *testing.T) {
	primary := &stubHost{name: "primary", tipErrs: []error{&esplora.HTTPError{StatusCode: 503}}}
	backup := &stubHost{name: "backup", tipErrs: []error{nil}, tip: 42}

	c := New(primary, backup)
	h, err := c.TipHeight(context.Background())
	if err != nil {
		t.Fatalf("TipHeight: %v", err)
	}
	if h != 42 {
		t.Fatalf("height = %d, want 42 (from backup)", h)
	}
	if c.Name() != "backup" {
		t.Fatalf("active host = %q, want backup", c.Name())
	}
	if !c.Degraded() {
		t.Fatal("expected Degraded() = true once away from the primary host")
	}
}

func TestNoFailoverOnNotFound(t *testing.T) {
	notFound := &esplora.HTTPError{StatusCode: 404}
	primary := &stubHost{name: "primary", tipErrs: []error{notFound}}
	backup := &stubHost{name: "backup", tipErrs: []error{nil}, tip: 1}

	c := New(primary, backup)
	_, err := c.TipHeight(context.Background())
	if !errors.Is(err, notFound) {
		t.Fatalf("expected the original 404 to propagate unchanged, got %v", err)
	}
	if c.Name() != "primary" {
		t.Fatalf("a plain 404 should not trigger failover, active = %q", c.Name())
	}
	if backup.calls != 0 {
		t.Fatalf("backup should not have been tried, calls = %d", backup.calls)
	}
}

func TestRecoversToPrimaryAfterCooldown(t *testing.T) {
	primary := &stubHost{name: "primary", tipErrs: []error{nil}, tip: 7}
	backup := &stubHost{name: "backup", tipErrs: []error{nil}, tip: 8}

	c := New(primary, backup)
	h, err := c.TipHeight(context.Background())
	if err != nil || h != 7 {
		t.Fatalf("expected primary to serve first request cleanly, got h=%d err=%v", h, err)
	}
	if c.Name() != "primary" {
		t.Fatalf("active = %q, want primary", c.Name())
	}
}

func TestPinDisablesFailover(t *testing.T) {
	primary := &stubHost{name: "primary", tipErrs: []error{&esplora.HTTPError{StatusCode: 503}}}
	backup := &stubHost{name: "backup", tipErrs: []error{nil}, tip: 99}

	c := New(primary, backup)
	if err := c.Pin("primary"); err != nil {
		t.Fatal(err)
	}
	_, err := c.TipHeight(context.Background())
	if err == nil {
		t.Fatal("expected the pinned primary's error to propagate without failover")
	}
	if backup.calls != 0 {
		t.Fatalf("pinned chain should never try backup, calls = %d", backup.calls)
	}
}

func TestErrUnsupportedDoesNotFailover(t *testing.T) {
	primary := &stubHost{name: "primary", tipErrs: []error{provider.ErrUnsupported}}
	backup := &stubHost{name: "backup", tipErrs: []error{nil}, tip: 1}

	c := New(primary, backup)
	_, err := c.TipHeight(context.Background())
	if !errors.Is(err, provider.ErrUnsupported) {
		t.Fatalf("expected ErrUnsupported to propagate as-is, got %v", err)
	}
	if backup.calls != 0 {
		t.Fatal("ErrUnsupported should not trigger failover")
	}
}
