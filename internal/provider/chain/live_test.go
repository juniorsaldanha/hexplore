package chain

import (
	"context"
	"errors"
	"testing"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/provider"
)

// liveStubHost embeds stubHost and additionally implements
// provider.LiveChainProvider, so Chain.Subscribe has something to delegate
// to.
type liveStubHost struct {
	*stubHost
	events chan provider.LiveEvent
	subErr error
}

func (s *liveStubHost) Subscribe(context.Context) (<-chan provider.LiveEvent, error) {
	if s.subErr != nil {
		return nil, s.subErr
	}
	return s.events, nil
}

func TestChainSubscribeDelegatesToActiveHost(t *testing.T) {
	live := &liveStubHost{stubHost: &stubHost{name: "live-host", tipErrs: []error{nil}}, events: make(chan provider.LiveEvent, 1)}
	c := New(live)

	ch, err := c.Subscribe(context.Background())
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	fees := domain.FeeTiers{HighSatVB: 42}
	live.events <- provider.LiveEvent{Fees: &fees}
	got := <-ch
	if got.Fees == nil || got.Fees.HighSatVB != 42 {
		t.Errorf("got = %+v, want the event passed through unchanged", got)
	}
}

func TestChainSubscribeUnsupportedHost(t *testing.T) {
	plain := &stubHost{name: "plain-host", tipErrs: []error{nil}}
	c := New(plain)

	_, err := c.Subscribe(context.Background())
	if err == nil {
		t.Fatal("expected an error when the active host has no live feed")
	}
	if !errors.Is(err, provider.ErrUnsupported) {
		t.Errorf("expected ErrUnsupported in the chain, got %v", err)
	}
}

func TestChainSubscribeUsesActiveHostAtCallTime(t *testing.T) {
	// primary fails its first call (TipHeight), which should move Chain's
	// active index to backup before Subscribe is ever called.
	primaryPlain := &stubHost{name: "primary-plain", tipErrs: []error{errors.New("boom")}}
	backupLive := &liveStubHost{stubHost: &stubHost{name: "backup-live", tipErrs: []error{nil}, tip: 1}, events: make(chan provider.LiveEvent, 1)}
	c := New(primaryPlain, backupLive)

	if _, err := c.TipHeight(context.Background()); err != nil {
		t.Fatalf("TipHeight: %v", err)
	}
	if c.Name() != "backup-live" {
		t.Fatalf("expected failover to backup-live, active = %q", c.Name())
	}

	if _, err := c.Subscribe(context.Background()); err != nil {
		t.Fatalf("expected Subscribe to delegate to the now-active live host, got %v", err)
	}
}
