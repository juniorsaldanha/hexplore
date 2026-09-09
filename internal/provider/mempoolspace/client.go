// Package mempoolspace embeds package esplora and layers mempool.space's
// /v1 extras on top: native block fees, mining pool attribution, and an
// exact next-block projection. It inherits every core method from esplora
// and overrides only what /v1 improves (docs/PLAN.md §1).
package mempoolspace

import (
	"context"
	"fmt"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/provider/esplora"
)

const DefaultBaseURL = "https://mempool.space/api"

type Provider struct {
	*esplora.Provider
}

func New(baseURL string) *Provider {
	return &Provider{Provider: esplora.New(baseURL)}
}

// Name inherits esplora.Provider's host-derived name — e.g. "mempool.space"
// or "mempool.emzy.de" — so hosts running the same software are still told
// apart in the status bar.

func (p *Provider) Caps() domain.Capabilities {
	c := p.Provider.Caps()
	c.BlockFees = true
	c.MiningPool = true
	c.NextBlockExact = true
	c.LiveBlockTxs = true
	c.WebSocket = true
	return c
}

func (p *Provider) BlockByHash(ctx context.Context, hash string) (domain.Block, error) {
	var b v1BlockJSON
	if err := p.Provider.GetJSON(ctx, "/v1/block/"+hash, &b); err != nil {
		return domain.Block{}, err
	}
	return b.toDomain(), nil
}

func (p *Provider) BlockByHeight(ctx context.Context, height int) (domain.Block, error) {
	body, err := p.Provider.Get(ctx, fmt.Sprintf("/block-height/%d", height))
	if err != nil {
		return domain.Block{}, err
	}
	return p.BlockByHash(ctx, string(body))
}

func (p *Provider) LatestBlocks(ctx context.Context, n int) ([]domain.Block, error) {
	var blocks []domain.Block
	path := "/v1/blocks"
	for len(blocks) < n {
		var page []v1BlockJSON
		if err := p.Provider.GetJSON(ctx, path, &page); err != nil {
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
		path = fmt.Sprintf("/v1/blocks/%d", last.Height-1)
	}
	return blocks, nil
}

func (p *Provider) FeeEstimates(ctx context.Context) (domain.FeeTiers, error) {
	var f v1FeesJSON
	if err := p.Provider.GetJSON(ctx, "/v1/fees/recommended", &f); err != nil {
		return domain.FeeTiers{}, err
	}
	return f.toDomain(), nil
}

func (p *Provider) NextBlocks(ctx context.Context, n int) ([]domain.ProjectedBlock, error) {
	var page []v1MempoolBlockJSON
	if err := p.Provider.GetJSON(ctx, "/v1/fees/mempool-blocks", &page); err != nil {
		return nil, err
	}
	if n > len(page) {
		n = len(page)
	}
	blocks := make([]domain.ProjectedBlock, n)
	for i := 0; i < n; i++ {
		blocks[i] = page[i].toDomain()
	}
	return blocks, nil
}
