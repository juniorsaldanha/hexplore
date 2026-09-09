// Package esplora talks to any Esplora-compatible REST API — the shared
// surface blockstream.info, mempool.space, and self-hosted esplora-electrs
// all serve identically. It never returns mining pool or native fee totals;
// callers needing those want package mempoolspace instead, which embeds
// this provider and layers its /v1 extras on top.
package esplora

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/juniorsaldanha/hexplore/internal/cache"
	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/enrich"
	"github.com/juniorsaldanha/hexplore/internal/provider"
)

const DefaultBaseURL = "https://mempool.space/api"

// HTTPError is a non-2xx response from the host. Package chain uses the
// status code to decide whether a failure is worth failing over (429, 5xx)
// or a legitimate application-level error (a plain 404 "not found") that
// should propagate as-is.
type HTTPError struct {
	Path       string
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("esplora: GET %s: status %d: %s", e.Path, e.StatusCode, e.Body)
}

type Provider struct {
	BaseURL string
	HTTP    *http.Client

	feeCache *cache.Immutable[string, domain.BlockFee] // keyed by block hash
}

func New(baseURL string) *Provider {
	return &Provider{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		HTTP:     &http.Client{Timeout: 10 * time.Second},
		feeCache: cache.NewImmutable[string, domain.BlockFee](),
	}
}

// Name is the base URL's host — e.g. "blockstream.info" — so the status bar
// can tell apart hosts that run identical software (docs/PLAN.md §2).
func (p *Provider) Name() string {
	if u, err := url.Parse(p.BaseURL); err == nil && u.Host != "" {
		return u.Host
	}
	return p.BaseURL
}

func (p *Provider) Caps() domain.Capabilities {
	return domain.Capabilities{
		BlockFees:      false,
		MiningPool:     false,
		NextBlockExact: false,
		AddressPrefix:  true,
		WebSocket:      false,
		LiveBlockTxs:   false,
		Testnet:        true,
		Signet:         true,
	}
}

// Get issues a raw GET against the provider's base URL. Exported so package
// mempoolspace, which embeds Provider, can reach endpoints outside the core
// Esplora surface without duplicating HTTP setup.
func (p *Provider) Get(ctx context.Context, path string) ([]byte, error) {
	return p.get(ctx, path)
}

// GetJSON is Get plus JSON decoding into v. See Get.
func (p *Provider) GetJSON(ctx context.Context, path string, v any) error {
	return p.getJSON(ctx, path, v)
}

func (p *Provider) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("esplora: GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("esplora: GET %s: read body: %w", path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{Path: path, StatusCode: resp.StatusCode, Body: string(body)}
	}
	return body, nil
}

func (p *Provider) getJSON(ctx context.Context, path string, v any) error {
	body, err := p.get(ctx, path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("esplora: GET %s: decode: %w", path, err)
	}
	return nil
}

func (p *Provider) TipHeight(ctx context.Context) (int, error) {
	body, err := p.get(ctx, "/blocks/tip/height")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(body)))
}

func (p *Provider) LatestBlocks(ctx context.Context, n int) ([]domain.Block, error) {
	var blocks []domain.Block
	path := "/blocks"
	for len(blocks) < n {
		var page []blockJSON
		if err := p.getJSON(ctx, path, &page); err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		for _, b := range page {
			blocks = append(blocks, b.toDomain())
			if len(blocks) >= n {
				break
			}
		}
		last := page[len(page)-1]
		path = fmt.Sprintf("/blocks/%d", last.Height-1)
	}
	for i := range blocks {
		p.attachFee(ctx, &blocks[i])
	}
	return blocks, nil
}

func (p *Provider) BlockByHeight(ctx context.Context, height int) (domain.Block, error) {
	body, err := p.get(ctx, fmt.Sprintf("/block-height/%d", height))
	if err != nil {
		return domain.Block{}, err
	}
	return p.BlockByHash(ctx, strings.TrimSpace(string(body)))
}

func (p *Provider) BlockByHash(ctx context.Context, hash string) (domain.Block, error) {
	var b blockJSON
	if err := p.getJSON(ctx, "/block/"+hash, &b); err != nil {
		return domain.Block{}, err
	}
	block := b.toDomain()
	p.attachFee(ctx, &block)
	return block, nil
}

func (p *Provider) BlockTxs(ctx context.Context, hash string, start int) ([]domain.Tx, error) {
	var page []txJSON
	if err := p.getJSON(ctx, fmt.Sprintf("/block/%s/txs/%d", hash, start), &page); err != nil {
		return nil, err
	}
	txs := make([]domain.Tx, len(page))
	for i, t := range page {
		txs[i] = t.toDomain()
	}
	return txs, nil
}

func (p *Provider) Tx(ctx context.Context, txid string) (domain.Tx, error) {
	var t txJSON
	if err := p.getJSON(ctx, "/tx/"+txid, &t); err != nil {
		return domain.Tx{}, err
	}
	return t.toDomain(), nil
}

func (p *Provider) Address(ctx context.Context, addr string) (domain.Address, error) {
	var a addressJSON
	if err := p.getJSON(ctx, "/address/"+addr, &a); err != nil {
		return domain.Address{}, err
	}
	return a.toDomain(), nil
}

func (p *Provider) AddressTxs(ctx context.Context, addr string, lastSeenTxID string) ([]domain.Tx, error) {
	path := "/address/" + addr + "/txs"
	if lastSeenTxID != "" {
		path = "/address/" + addr + "/txs/chain/" + lastSeenTxID
	}
	var page []txJSON
	if err := p.getJSON(ctx, path, &page); err != nil {
		return nil, err
	}
	txs := make([]domain.Tx, len(page))
	for i, t := range page {
		txs[i] = t.toDomain()
	}
	return txs, nil
}

func (p *Provider) AddressPrefix(ctx context.Context, prefix string) ([]string, error) {
	var names []string
	if err := p.getJSON(ctx, "/address-prefix/"+prefix, &names); err != nil {
		if httpErr, ok := errors.AsType[*HTTPError](err); ok && httpErr.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("esplora: address-prefix: %w", provider.ErrUnsupported)
		}
		return nil, err
	}
	return names, nil
}

func (p *Provider) Mempool(ctx context.Context) (domain.MempoolState, error) {
	var m mempoolJSON
	if err := p.getJSON(ctx, "/mempool", &m); err != nil {
		return domain.MempoolState{}, err
	}
	return m.toDomain(), nil
}

func (p *Provider) FeeEstimates(ctx context.Context) (domain.FeeTiers, error) {
	var m map[string]float64
	if err := p.getJSON(ctx, "/fee-estimates", &m); err != nil {
		return domain.FeeTiers{}, err
	}
	return domain.FeeTiers{
		HighSatVB:    m["1"],
		AvgSatVB:     m["3"],
		LowSatVB:     m["6"],
		EconomySatVB: m["144"],
	}, nil
}

func (p *Provider) NextBlocks(ctx context.Context, n int) ([]domain.ProjectedBlock, error) {
	m, err := p.Mempool(ctx)
	if err != nil {
		return nil, err
	}
	return enrich.ProjectNextBlocks(m, n), nil
}

// ProjectedBlockTxs is a mempool.space /v1/ws feature; plain Esplora has no
// per-tx projected-block feed at all.
func (p *Provider) ProjectedBlockTxs(ctx context.Context, blockIndex int) ([]domain.ProjectedTx, error) {
	return nil, fmt.Errorf("esplora: projected block txs: %w", provider.ErrUnsupported)
}

// attachFee derives a block's fee total from its coinbase output (docs/PLAN.md
// §4) and caches the result forever, keyed by hash — both source requests are
// on immutable data. Errors are swallowed: a missing fee panel beats a broken
// block list.
func (p *Provider) attachFee(ctx context.Context, b *domain.Block) {
	if fee, ok := p.feeCache.Get(b.Hash); ok {
		b.Fee = &fee
		return
	}
	txidBody, err := p.get(ctx, fmt.Sprintf("/block/%s/txid/0", b.Hash))
	if err != nil {
		return
	}
	coinbaseTxID := strings.TrimSpace(string(txidBody))
	coinbase, err := p.Tx(ctx, coinbaseTxID)
	if err != nil {
		return
	}
	var voutSum int64
	for _, v := range coinbase.Vout {
		voutSum += v.Value
	}
	fee := enrich.BlockFee(voutSum, b.Height, b.Weight, b.TxCount)
	p.feeCache.Set(b.Hash, fee)
	b.Fee = &fee
}
