package poll

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/provider"
)

// stubChain is a minimal provider.ChainProvider that errors on everything —
// tests only care about whether startLive wires up the live channel, not
// about REST fetch behavior (already covered by other packages' tests).
type stubChain struct{}

func (stubChain) Name() string                           { return "stub" }
func (stubChain) Caps() domain.Capabilities              { return domain.Capabilities{} }
func (stubChain) TipHeight(context.Context) (int, error) { return 0, errors.New("n/a") }
func (stubChain) LatestBlocks(context.Context, int) ([]domain.Block, error) {
	return nil, errors.New("n/a")
}
func (stubChain) BlockByHeight(context.Context, int) (domain.Block, error) {
	return domain.Block{}, errors.New("n/a")
}
func (stubChain) BlockByHash(context.Context, string) (domain.Block, error) {
	return domain.Block{}, errors.New("n/a")
}
func (stubChain) BlockTxs(context.Context, string, int) ([]domain.Tx, error) {
	return nil, errors.New("n/a")
}
func (stubChain) Tx(context.Context, string) (domain.Tx, error) {
	return domain.Tx{}, errors.New("n/a")
}
func (stubChain) Address(context.Context, string) (domain.Address, error) {
	return domain.Address{}, errors.New("n/a")
}
func (stubChain) AddressTxs(context.Context, string, string) ([]domain.Tx, error) {
	return nil, errors.New("n/a")
}
func (stubChain) AddressPrefix(context.Context, string) ([]string, error) {
	return nil, errors.New("n/a")
}
func (stubChain) Mempool(context.Context) (domain.MempoolState, error) {
	return domain.MempoolState{}, errors.New("n/a")
}
func (stubChain) FeeEstimates(context.Context) (domain.FeeTiers, error) {
	return domain.FeeTiers{}, errors.New("n/a")
}
func (stubChain) NextBlocks(context.Context, int) ([]domain.ProjectedBlock, error) {
	return nil, errors.New("n/a")
}
func (stubChain) ProjectedBlockTxs(context.Context, int) ([]domain.ProjectedTx, error) {
	return nil, errors.New("n/a")
}

// liveStubChain additionally implements provider.LiveChainProvider.
type liveStubChain struct {
	stubChain
	events chan provider.LiveEvent
}

func (l liveStubChain) Subscribe(context.Context) (<-chan provider.LiveEvent, error) {
	return l.events, nil
}

func TestStartLiveNoOpForNonLiveProvider(t *testing.T) {
	ctx := t.Context()
	s := New(stubChain{}, nil, "USD", 1, Intervals{Tip: time.Hour, Mempool: time.Hour, Fees: time.Hour, Price: time.Hour})
	// startLive should simply do nothing (no panic, no goroutine spun up
	// that sends anything) when Chain isn't a LiveChainProvider.
	s.startLive(ctx)
	select {
	case msg := <-s.msgs:
		t.Fatalf("expected no message from a non-live provider, got %#v", msg)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestStartLiveForwardsBlocksNextBlocksAndFees(t *testing.T) {
	events := make(chan provider.LiveEvent, 3)
	ctx := t.Context()

	s := New(liveStubChain{events: events}, nil, "USD", 1, Intervals{Tip: time.Hour, Mempool: time.Hour, Fees: time.Hour, Price: time.Hour})
	s.startLive(ctx)

	fees := domain.FeeTiers{HighSatVB: 99}
	events <- provider.LiveEvent{Blocks: []domain.Block{{Height: 1}}}
	events <- provider.LiveEvent{NextBlocks: []domain.ProjectedBlock{{VSize: 1}}}
	events <- provider.LiveEvent{Fees: &fees}

	got := map[string]bool{}
	deadline := time.After(2 * time.Second)
	for len(got) < 3 {
		select {
		case msg := <-s.msgs:
			switch m := msg.(type) {
			case BlocksMsg:
				if len(m.Blocks) != 1 || m.Blocks[0].Height != 1 {
					t.Errorf("unexpected BlocksMsg: %+v", m)
				}
				got["blocks"] = true
			case NextBlocksMsg:
				got["nextblocks"] = true
			case FeesMsg:
				if m.Tiers.HighSatVB != 99 {
					t.Errorf("unexpected FeesMsg: %+v", m)
				}
				got["fees"] = true
			default:
				t.Fatalf("unexpected message type %T", msg)
			}
		case <-deadline:
			t.Fatalf("timed out waiting for all 3 live-forwarded messages, got %v", got)
		}
	}
}
