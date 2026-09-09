// Package provider defines the two interfaces hexplore's UI and enrich
// layers depend on. Concrete adapters (esplora, mempoolspace, price/*) live
// in subpackages and are never imported outside this tree.
package provider

import (
	"context"
	"errors"

	"github.com/juniorsaldanha/hexplore/internal/domain"
)

// ErrUnsupported is returned when a provider can't fulfill a request because
// the underlying API doesn't offer it (e.g. address-prefix autocomplete on
// plain Esplora). Callers render "—" with a reason instead of failing.
var ErrUnsupported = errors.New("unsupported by provider")

type ChainProvider interface {
	Name() string
	Caps() domain.Capabilities

	TipHeight(ctx context.Context) (int, error)
	LatestBlocks(ctx context.Context, n int) ([]domain.Block, error)
	BlockByHeight(ctx context.Context, height int) (domain.Block, error)
	BlockByHash(ctx context.Context, hash string) (domain.Block, error)
	BlockTxs(ctx context.Context, hash string, start int) ([]domain.Tx, error)

	Tx(ctx context.Context, txid string) (domain.Tx, error)
	Address(ctx context.Context, addr string) (domain.Address, error)
	AddressTxs(ctx context.Context, addr string, lastSeenTxID string) ([]domain.Tx, error)
	AddressPrefix(ctx context.Context, prefix string) ([]string, error) // may return ErrUnsupported

	Mempool(ctx context.Context) (domain.MempoolState, error)
	FeeEstimates(ctx context.Context) (domain.FeeTiers, error)
	NextBlocks(ctx context.Context, n int) ([]domain.ProjectedBlock, error)

	// ProjectedBlockTxs is the live per-tx feed for a not-yet-mined block
	// (blockIndex 0 = next), powering a treemap-style visualization. May
	// return ErrUnsupported — only mempool.space's /v1/ws offers this.
	ProjectedBlockTxs(ctx context.Context, blockIndex int) ([]domain.ProjectedTx, error)
}

type PriceProvider interface {
	Name() string
	Spot(ctx context.Context, currency string) (domain.Price, error)
	History24h(ctx context.Context, currency string) ([]float64, error) // sparkline series
	SupportedCurrencies() []string
}

// LiveEvent is a partial update from a push feed — only the non-nil/non-
// empty fields carry new data; a receiver applies whichever are set.
type LiveEvent struct {
	Blocks     []domain.Block
	NextBlocks []domain.ProjectedBlock
	Fees       *domain.FeeTiers
}

// LiveChainProvider is an optional capability — checked with a type
// assertion rather than folded into ChainProvider, since it's tied to a
// specific transport (mempool.space's WebSocket) most providers don't
// have. Where available, poll.Scheduler layers it on top of REST polling
// as a lower-latency addition, not a replacement: a push feed that drops
// for good shouldn't mean losing updates for the rest of the session.
type LiveChainProvider interface {
	ChainProvider
	// Subscribe starts a live feed and returns a channel of updates. The
	// feed reconnects internally (with backoff) on drops; the channel
	// closes only once ctx is cancelled.
	Subscribe(ctx context.Context) (<-chan LiveEvent, error)
}
