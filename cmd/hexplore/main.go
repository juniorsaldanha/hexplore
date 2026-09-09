// hexplore is a read-only Bitcoin block explorer TUI. See docs/PLAN.md.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/juniorsaldanha/hexplore/internal/app"
	"github.com/juniorsaldanha/hexplore/internal/cache"
	"github.com/juniorsaldanha/hexplore/internal/config"
	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/poll"
	"github.com/juniorsaldanha/hexplore/internal/provider"
	"github.com/juniorsaldanha/hexplore/internal/provider/chain"
	"github.com/juniorsaldanha/hexplore/internal/provider/price/coingecko"
	"github.com/juniorsaldanha/hexplore/internal/ui/components"
)

// version is set at build time via -ldflags "-X main.version=..." (see
// .goreleaser.yml) — "dev" for a plain `go build`/`go run`.
var version = "dev"

func main() {
	args := os.Args[1:]
	if slices.Contains(args, "--version") || slices.Contains(args, "-v") {
		fmt.Println("hexplore " + version)
		return
	}
	jsonMode := slices.Contains(args, "--json")
	args = slices.DeleteFunc(args, func(s string) bool { return s == "--json" })

	path := config.DefaultPath()
	if len(args) > 0 {
		path = args[0]
	}
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hexplore: config:", err)
		os.Exit(1)
	}

	c, err := chain.NewFromHosts(cfg.Provider.Hosts, cfg.Provider.Network)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hexplore:", err)
		os.Exit(1)
	}
	if !cfg.Provider.Failover {
		if err := c.Pin(c.Hosts()[0]); err != nil {
			fmt.Fprintln(os.Stderr, "hexplore:", err)
			os.Exit(1)
		}
	}

	prices := newPriceProvider(cfg.Price)

	if jsonMode {
		if err := runJSON(c, prices, cfg); err != nil {
			fmt.Fprintln(os.Stderr, "hexplore:", err)
			os.Exit(1)
		}
		return
	}

	watchlist, err := cache.OpenWatchlist(filepath.Join(config.ExpandPath(cfg.Cache.Path), "watchlist.db"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "hexplore: watchlist disabled:", err)
	} else {
		defer watchlist.Close()
	}

	m := app.New(c, prices, app.Config{
		Network:      cfg.Provider.Network,
		Currency:     cfg.Price.Currency,
		Currencies:   cfg.Price.Currencies,
		NumBlocks:    cfg.Display.Blocks,
		AssumedVSize: cfg.Display.AssumedTxVSize,
		Intervals: poll.Intervals{
			Tip:     config.Duration(cfg.Refresh.Tip, poll.DefaultIntervals().Tip),
			Mempool: config.Duration(cfg.Refresh.Mempool, poll.DefaultIntervals().Mempool),
			Fees:    config.Duration(cfg.Refresh.Fees, poll.DefaultIntervals().Fees),
			Price:   config.Duration(cfg.Refresh.Price, poll.DefaultIntervals().Price),
		},
		Watchlist: watchlist,
		GraphStyle: func() components.GraphStyle {
			style, _ := components.ParseGraphStyle(cfg.Display.GraphStyle)
			return style
		}(),
	})

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "hexplore:", err)
		os.Exit(1)
	}
}

func newPriceProvider(cfg config.PriceConfig) provider.PriceProvider {
	if cfg.Provider == "none" {
		return nil
	}
	// Only coingecko is implemented so far; anything else falls back to it
	// rather than failing startup over a config typo.
	p := coingecko.New(coingecko.DefaultBaseURL)
	if key, err := config.APIKey("coingecko"); err == nil {
		p.APIKey = key
	}
	return p
}

// jsonSnapshot is hexplore --json's non-interactive output — one fetch of
// everything the dashboard shows, printed once instead of rendered live.
type jsonSnapshot struct {
	Provider   string                  `json:"provider"`
	Network    string                  `json:"network"`
	TipHeight  int                     `json:"tip_height"`
	Blocks     []domain.Block          `json:"blocks"`
	Mempool    domain.MempoolState     `json:"mempool"`
	Fees       domain.FeeTiers         `json:"fees"`
	NextBlocks []domain.ProjectedBlock `json:"next_blocks"`
	Price      *domain.Price           `json:"price,omitempty"`
}

func runJSON(c provider.ChainProvider, prices provider.PriceProvider, cfg config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	blocks, err := c.LatestBlocks(ctx, cfg.Display.Blocks)
	if err != nil {
		return fmt.Errorf("blocks: %w", err)
	}
	mempool, err := c.Mempool(ctx)
	if err != nil {
		return fmt.Errorf("mempool: %w", err)
	}
	fees, err := c.FeeEstimates(ctx)
	if err != nil {
		return fmt.Errorf("fees: %w", err)
	}
	next, err := c.NextBlocks(ctx, 3)
	if err != nil {
		return fmt.Errorf("next blocks: %w", err)
	}

	snap := jsonSnapshot{
		Provider:   c.Name(),
		Network:    cfg.Provider.Network,
		Blocks:     blocks,
		Mempool:    mempool,
		Fees:       fees,
		NextBlocks: next,
	}
	if len(blocks) > 0 {
		snap.TipHeight = blocks[0].Height
	}
	if prices != nil {
		if p, err := prices.Spot(ctx, cfg.Price.Currency); err == nil {
			snap.Price = &p
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(snap)
}
