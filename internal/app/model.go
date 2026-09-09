// Package app is the bubbletea root model: view routing, the navigation
// stack, and the global keymap. It is the only package that wires provider
// + poll + ui together.
package app

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/juniorsaldanha/hexplore/internal/cache"
	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/enrich"
	"github.com/juniorsaldanha/hexplore/internal/poll"
	"github.com/juniorsaldanha/hexplore/internal/provider"
	"github.com/juniorsaldanha/hexplore/internal/provider/chain"
	"github.com/juniorsaldanha/hexplore/internal/ui/components"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
	"github.com/juniorsaldanha/hexplore/internal/ui/views"
	"github.com/juniorsaldanha/hexplore/internal/wallet"
)

type inputMode int

const (
	modeNormal inputMode = iota
	modeSearch
	modeCommand
	// modeWalletScriptType guides the user through resolving a plain
	// xpub/tpub's genuine ambiguity — it fits both BIP44 (legacy) and
	// BIP86 (taproot), see wallet.AmbiguousScriptType — before scanning
	// it, rather than silently guessing one.
	modeWalletScriptType
)

// walletScriptTypeChoices is what modeWalletScriptType lets the user pick
// between — the two conventions a plain xpub/tpub prefix is actually
// compatible with.
var walletScriptTypeChoices = []wallet.ScriptType{wallet.Legacy, wallet.Taproot}

type Config struct {
	Network      string
	Currency     string
	Currencies   []string // cycled with `p`; Currency is used if empty
	NumBlocks    int
	AssumedVSize int
	Intervals    poll.Intervals   // zero value falls back to poll.DefaultIntervals()
	Watchlist    *cache.Watchlist // nil disables the `w` watchlist screen
	GraphStyle   components.GraphStyle
}

type Model struct {
	chain    provider.ChainProvider
	chainCtl *chain.Chain           // non-nil when chain is a fallback Chain — enables :provider and the degraded indicator
	prices   provider.PriceProvider // nil disables the price panel
	sched    *poll.Scheduler
	ctx      context.Context
	cancel   context.CancelFunc
	keymap   KeyMap
	theme    theme.Theme
	cfg      Config

	width, height int

	// dashboard state — the scheduler is the sole source, these are its
	// last known good values (the in-memory "volatile cache").
	tipHeight    int
	blocks       []domain.Block
	mempool      domain.MempoolState
	fees         domain.FeeTiers
	nextBlocks   []domain.ProjectedBlock
	price        domain.Price
	priceHistory []float64
	cursor       int

	lastGood map[string]time.Time
	lastErr  map[string]error

	currencyIdx int
	units       views.Unit
	graphStyle  components.GraphStyle
	fps         fpsTracker

	stack []screen

	mode         inputMode
	searchInput  textinput.Model
	searchErr    string
	commandInput textinput.Model
	statusMsg    string

	// pendingWalletKey/walletScriptTypeCursor back modeWalletScriptType —
	// the raw xpub/tpub awaiting a derivation choice, and which of
	// walletScriptTypeChoices is currently highlighted.
	pendingWalletKey       string
	walletScriptTypeCursor int
	// resolvedWalletScriptTypes remembers a chosen derivation for the
	// session, keyed by the raw extended-key string, so re-opening the
	// same ambiguous xpub/tpub never re-prompts twice in one run — see
	// also cache.WatchedWallet.ScriptType for the persisted form.
	resolvedWalletScriptTypes map[string]wallet.ScriptType

	quitting bool
}

func New(chainProvider provider.ChainProvider, prices provider.PriceProvider, cfg Config) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	iv := cfg.Intervals
	if iv == (poll.Intervals{}) {
		iv = poll.DefaultIntervals()
	}
	sched := poll.New(chainProvider, prices, cfg.Currency, cfg.NumBlocks, iv)

	search := textinput.New()
	search.Placeholder = "height / hash / txid / address"
	search.Prompt = "/ "

	cmd := textinput.New()
	cmd.Prompt = ": "

	var chainCtl *chain.Chain
	if c, ok := chainProvider.(*chain.Chain); ok {
		chainCtl = c
	}

	return &Model{
		chain:                     chainProvider,
		chainCtl:                  chainCtl,
		prices:                    prices,
		sched:                     sched,
		ctx:                       ctx,
		cancel:                    cancel,
		keymap:                    DefaultKeyMap(),
		theme:                     theme.Active(),
		cfg:                       cfg,
		lastGood:                  map[string]time.Time{},
		lastErr:                   map[string]error{},
		resolvedWalletScriptTypes: map[string]wallet.ScriptType{},
		searchInput:               search,
		commandInput:              cmd,
		graphStyle:                cfg.GraphStyle,
	}
}

func (m *Model) Init() tea.Cmd {
	m.sched.Start(m.ctx)
	return tea.Batch(m.sched.Listen(), frameTick())
}

// targetFPS drives the render loop independent of data updates — mostly so
// the age clocks and any future animation feel alive (docs/PLAN.md §11:
// "a clock that visibly runs is most of what makes a dashboard feel
// alive"), and so the fps counter has something continuous to measure.
const targetFPS = 60

var frameInterval = time.Second / targetFPS

type frameTickMsg time.Time

func frameTick() tea.Cmd {
	return tea.Tick(frameInterval, func(t time.Time) tea.Msg { return frameTickMsg(t) })
}

// --- Update ---

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case poll.BlocksMsg:
		m.record("blocks", msg.Err)
		if msg.Err == nil {
			m.blocks = msg.Blocks
			if len(m.blocks) > 0 {
				m.tipHeight = m.blocks[0].Height
			}
			if m.cursor >= len(m.blocks) {
				m.cursor = max(len(m.blocks)-1, 0)
			}
		}
		return m, m.sched.Listen()

	case poll.MempoolMsg:
		m.record("mempool", msg.Err)
		if msg.Err == nil {
			m.mempool = msg.State
		}
		return m, m.sched.Listen()

	case poll.FeesMsg:
		m.record("fees", msg.Err)
		if msg.Err == nil {
			m.fees = msg.Tiers
		}
		return m, m.sched.Listen()

	case poll.NextBlocksMsg:
		m.record("nextblocks", msg.Err)
		if msg.Err == nil {
			m.nextBlocks = msg.Blocks
		}
		return m, m.sched.Listen()

	case poll.PriceMsg:
		m.record("price", msg.Err)
		if msg.Err == nil {
			m.price = msg.Price
		}
		return m, m.sched.Listen()

	case poll.PriceHistoryMsg:
		m.record("price", msg.Err)
		if msg.Err == nil {
			m.priceHistory = msg.Series
		}
		return m, m.sched.Listen()

	case blockLoadedMsg:
		if len(m.stack) > 0 {
			top := &m.stack[len(m.stack)-1]
			top.loading = false
			top.err = msg.err
			if msg.err == nil {
				top.block, top.blockTxs, top.blockPage = msg.block, msg.txs, 0
				top.blockTreemapLoading = true
				return m, fetchBlockTreemap(m.ctx, m.chain, msg.block.Hash, msg.block.TxCount)
			}
		}
		return m, nil

	case blockTreemapMsg:
		if len(m.stack) > 0 {
			top := &m.stack[len(m.stack)-1]
			top.blockTreemapLoading = false
			top.blockTreemapTxs = msg.txs
			top.blockTreemapTruncated = msg.truncated
		}
		return m, nil

	case blockPageMsg:
		if len(m.stack) > 0 {
			top := &m.stack[len(m.stack)-1]
			top.err = msg.err
			if msg.err == nil {
				top.blockTxs, top.blockPage, top.cursor = msg.txs, msg.page, 0
			}
		}
		return m, nil

	case txLoadedMsg:
		if len(m.stack) > 0 {
			top := &m.stack[len(m.stack)-1]
			top.loading = false
			top.err = msg.err
			if msg.err == nil {
				top.tx = msg.tx
			}
		}
		return m, nil

	case addressLoadedMsg:
		if len(m.stack) > 0 {
			top := &m.stack[len(m.stack)-1]
			top.loading = false
			top.err = msg.err
			if msg.err == nil {
				top.address, top.addressTxs = msg.address, msg.txs
				// Encoding-derived features (SegWit/Taproot) don't need
				// tx data — show them immediately rather than waiting on
				// the history fetch below for something already known.
				top.addressFeatures = enrich.AddressFeatures(msg.address.Address, nil)
				top.addressHistoryLoading = true
				return m, fetchAddressHistory(m.ctx, m.chain, msg.address.Address, msg.address.BalanceSats())
			}
		}
		return m, nil

	case addressHistoryMsg:
		if len(m.stack) > 0 {
			top := &m.stack[len(m.stack)-1]
			top.addressHistoryLoading = false
			top.addressHistory = msg.series
			top.addressHistoryTruncated = msg.truncated
			top.addressFeatures = msg.features
		}
		return m, nil

	case walletScanMsg:
		if len(m.stack) > 0 {
			top := &m.stack[len(m.stack)-1]
			top.loading = false
			top.err = msg.err
			if msg.err == nil {
				top.wallet = msg.wallet
			}
		}
		return m, nil

	case watchlistStatsMsg:
		if len(m.stack) > 0 {
			top := &m.stack[len(m.stack)-1]
			if top.kind == screenWatchlist {
				top.watchlistLoading = false
				top.watchlistAddrStats = msg.addrStats
				top.watchlistWalletStats = msg.walletStats
			}
		}
		return m, nil

	case liveBlockLoadedMsg:
		if len(m.stack) > 0 {
			top := &m.stack[len(m.stack)-1]
			top.loading = false
			if msg.err == nil {
				top.liveBlockTxs = msg.txs
				top.err = nil
			} else if len(top.liveBlockTxs) == 0 {
				// Nothing on screen yet — show the error. Once we have a
				// good snapshot, a transient refresh failure keeps showing
				// the last one rather than blanking a "live" screen.
				top.err = msg.err
			}
		}
		return m, nil

	case liveBlockTickMsg:
		// The screen may have been popped since this tick was scheduled —
		// only keep refreshing (and re-arm the next tick) while it's still
		// the one on top; otherwise let the chain die out quietly.
		if len(m.stack) == 0 || m.stack[len(m.stack)-1].kind != screenLiveBlock {
			return m, nil
		}
		return m, tea.Batch(fetchProjectedBlockTxs(m.ctx, m.chain, 0), liveBlockTick())

	case frameTickMsg:
		m.fps.record(time.Time(msg))
		return m, frameTick()
	}
	return m, nil
}

func (m *Model) record(stream string, err error) {
	if err == nil {
		m.lastGood[stream] = time.Now()
		delete(m.lastErr, stream)
	} else {
		m.lastErr[stream] = err
	}
}

// currentCurrency is cfg.Currencies[currencyIdx], falling back to the
// single configured Currency when Currencies is empty.
func (m *Model) currentCurrency() string {
	if len(m.cfg.Currencies) == 0 {
		return m.cfg.Currency
	}
	return m.cfg.Currencies[m.currencyIdx%len(m.cfg.Currencies)]
}

func (m *Model) cycleCurrency() {
	if len(m.cfg.Currencies) < 2 {
		m.statusMsg = "only one currency configured"
		return
	}
	m.currencyIdx = (m.currencyIdx + 1) % len(m.cfg.Currencies)
	cur := m.currentCurrency()
	m.sched.SetCurrency(cur)
	m.sched.RefreshPrice(m.ctx)
	m.statusMsg = "currency: " + cur
}

func (m *Model) cycleTheme() {
	m.theme = theme.Next(m.theme)
	m.statusMsg = "theme: " + m.theme.Name
}

func (m *Model) cycleUnit() {
	m.units = m.units.Next()
	m.statusMsg = "units: " + m.units.String()
}

func (m *Model) openWatchlist() tea.Cmd {
	if m.cfg.Watchlist == nil {
		m.statusMsg = "watchlist unavailable (no cache dir configured)"
		return nil
	}
	entries, err := m.cfg.Watchlist.List()
	if err != nil {
		m.statusMsg = "watchlist: " + err.Error()
		return nil
	}
	wallets, err := m.cfg.Watchlist.ListWallets()
	if err != nil {
		m.statusMsg = "watchlist: " + err.Error()
		return nil
	}
	m.push(screen{kind: screenWatchlist, watchlist: entries, watchlistWallets: wallets, watchlistLoading: true})
	return fetchWatchlistStats(m.ctx, m.chain, entries, wallets, m.snapshotResolvedWalletScriptTypes())
}

// snapshotResolvedWalletScriptTypes copies resolvedWalletScriptTypes so a
// background fetch never reads the live map while the main loop might
// still be writing to it.
func (m *Model) snapshotResolvedWalletScriptTypes() map[string]wallet.ScriptType {
	out := make(map[string]wallet.ScriptType, len(m.resolvedWalletScriptTypes))
	for k, v := range m.resolvedWalletScriptTypes {
		out[k] = v
	}
	return out
}

// openWallet pushes a wallet-scan screen for the given extended public
// key — unless its prefix is genuinely ambiguous (a plain xpub/tpub fits
// both BIP44 and BIP86), in which case it asks first rather than guessing.
func (m *Model) openWallet(key string) (tea.Model, tea.Cmd) {
	if st, ok := m.savedWalletScriptType(key); ok {
		m.push(screen{kind: screenWallet, loading: true})
		return m, fetchWalletScanAs(m.ctx, m.chain, key, st)
	}
	if wallet.AmbiguousScriptType(key) {
		m.mode = modeWalletScriptType
		m.pendingWalletKey = key
		m.walletScriptTypeCursor = 0
		return m, nil
	}
	m.push(screen{kind: screenWallet, loading: true})
	return m, fetchWalletScan(m.ctx, m.chain, key)
}

// savedWalletScriptType looks up a previously resolved derivation choice
// for key — the in-session cache first, then the persisted watchlist
// entry (if key is watched and was saved with one) — so an ambiguous
// xpub/tpub is never re-prompted once it's been answered.
func (m *Model) savedWalletScriptType(key string) (wallet.ScriptType, bool) {
	if st, ok := m.resolvedWalletScriptTypes[key]; ok {
		return st, true
	}
	if m.cfg.Watchlist == nil {
		return 0, false
	}
	wallets, err := m.cfg.Watchlist.ListWallets()
	if err != nil {
		return 0, false
	}
	for _, w := range wallets {
		if w.Key == key && w.ScriptType != "" {
			if st, ok := wallet.ParseScriptTypeName(w.ScriptType); ok {
				return st, true
			}
		}
	}
	return 0, false
}

// saveWalletScriptType remembers a resolved derivation choice for the rest
// of the session, and persists it onto the watchlist entry if key is
// already watched (preserving its label/added-at).
func (m *Model) saveWalletScriptType(key string, st wallet.ScriptType) {
	m.resolvedWalletScriptTypes[key] = st
	if m.cfg.Watchlist == nil {
		return
	}
	wallets, err := m.cfg.Watchlist.ListWallets()
	if err != nil {
		return
	}
	for _, w := range wallets {
		if w.Key == key {
			w.ScriptType = st.Name()
			_ = m.cfg.Watchlist.AddWallet(w)
			return
		}
	}
}

// renderWalletScriptTypePrompt is the small j/k-and-enter selection
// modeWalletScriptType shows above the current screen — the same overlay
// pattern as the search/command input bars, not a full screen of its own.
func (m *Model) renderWalletScriptTypePrompt() string {
	dim := lipgloss.NewStyle().Foreground(m.theme.Dim)
	selected := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Accent)

	lines := []string{
		lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true).Render("This key's prefix works for more than one derivation — which is it?"),
		dim.Render(truncateForPrompt(m.pendingWalletKey)),
	}
	for i, st := range walletScriptTypeChoices {
		line := "  " + st.String()
		if i == m.walletScriptTypeCursor {
			line = selected.Render("> " + st.String())
		}
		lines = append(lines, line)
	}
	lines = append(lines, dim.Render("j/k select   ↵ confirm   Esc cancel"))
	return strings.Join(lines, "\n")
}

// truncateForPrompt keeps a long xpub from wrapping oddly in the prompt.
func truncateForPrompt(s string) string {
	const max = 60
	if len(s) <= max {
		return s
	}
	half := (max - 3) / 2
	return s[:half] + "..." + s[len(s)-half:]
}

func (m *Model) handleWalletScriptTypeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.pendingWalletKey = ""
		return m, nil
	case "j", "down":
		if m.walletScriptTypeCursor < len(walletScriptTypeChoices)-1 {
			m.walletScriptTypeCursor++
		}
		return m, nil
	case "k", "up":
		if m.walletScriptTypeCursor > 0 {
			m.walletScriptTypeCursor--
		}
		return m, nil
	case "enter":
		key := m.pendingWalletKey
		st := walletScriptTypeChoices[m.walletScriptTypeCursor]
		m.mode = modeNormal
		m.pendingWalletKey = ""
		m.saveWalletScriptType(key, st)
		m.push(screen{kind: screenWallet, loading: true})
		return m, fetchWalletScanAs(m.ctx, m.chain, key, st)
	}
	return m, nil
}

// openLiveBlock pushes the live next-block treemap screen. Unsupported
// providers still get a screen (rendered with a plain "unavailable"
// message) rather than a silent no-op or a doomed-to-fail fetch.
func (m *Model) openLiveBlock() (tea.Model, tea.Cmd) {
	if !m.chain.Caps().LiveBlockTxs {
		m.push(screen{kind: screenLiveBlock})
		return m, nil
	}
	m.push(screen{kind: screenLiveBlock, loading: true})
	return m, tea.Batch(fetchProjectedBlockTxs(m.ctx, m.chain, 0), liveBlockTick())
}

// liveBlockRefreshInterval is how often the live-block screen re-fetches
// while it's open — fast enough to feel "live" without re-dialing the
// WebSocket every second.
const liveBlockRefreshInterval = 10 * time.Second

type liveBlockTickMsg struct{}

func liveBlockTick() tea.Cmd {
	return tea.Tick(liveBlockRefreshInterval, func(time.Time) tea.Msg { return liveBlockTickMsg{} })
}

// refreshWatchlistScreen re-reads the store into the top-of-stack watchlist
// screen, if there is one — called after :watch/:unwatch/`d` so the list
// stays in sync with what was just changed.
func (m *Model) refreshWatchlistScreen() tea.Cmd {
	if len(m.stack) == 0 || m.stack[len(m.stack)-1].kind != screenWatchlist || m.cfg.Watchlist == nil {
		return nil
	}
	entries, err := m.cfg.Watchlist.List()
	if err != nil {
		m.statusMsg = "watchlist: " + err.Error()
		return nil
	}
	wallets, err := m.cfg.Watchlist.ListWallets()
	if err != nil {
		m.statusMsg = "watchlist: " + err.Error()
		return nil
	}
	top := &m.stack[len(m.stack)-1]
	top.watchlist = entries
	top.watchlistWallets = wallets
	top.watchlistLoading = true
	total := len(entries) + len(wallets)
	if top.cursor >= total {
		top.cursor = max(total-1, 0)
	}
	return fetchWatchlistStats(m.ctx, m.chain, entries, wallets, m.snapshotResolvedWalletScriptTypes())
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeSearch:
		return m.handleSearchKey(msg)
	case modeCommand:
		return m.handleCommandKey(msg)
	case modeWalletScriptType:
		return m.handleWalletScriptTypeKey(msg)
	}

	switch {
	case keyMatches(msg, m.keymap.Quit):
		m.quitting = true
		m.cancel()
		return m, tea.Quit
	case keyMatches(msg, m.keymap.Search):
		m.mode = modeSearch
		m.searchInput.SetValue("")
		m.searchErr = ""
		m.searchInput.Focus()
		return m, textinput.Blink
	case keyMatches(msg, m.keymap.Command):
		m.mode = modeCommand
		m.commandInput.SetValue("")
		m.commandInput.Focus()
		return m, textinput.Blink
	case keyMatches(msg, m.keymap.Refresh):
		m.sched.Refresh(m.ctx)
		return m, nil
	case keyMatches(msg, m.keymap.HelpKey):
		m.push(screen{kind: screenHelp})
		return m, nil
	case keyMatches(msg, m.keymap.Config):
		m.push(screen{kind: screenConfig})
		return m, nil
	case keyMatches(msg, m.keymap.Yank):
		m.yank()
		return m, nil
	case keyMatches(msg, m.keymap.Open):
		m.openInBrowser()
		return m, nil
	case keyMatches(msg, m.keymap.Unit):
		m.cycleUnit()
		return m, nil
	case keyMatches(msg, m.keymap.Theme):
		m.cycleTheme()
		return m, nil
	case keyMatches(msg, m.keymap.Watchlist):
		return m, m.openWatchlist()
	case keyMatches(msg, m.keymap.LiveBlock):
		return m.openLiveBlock()
	case keyMatches(msg, m.keymap.Back):
		m.pop()
		return m, nil
	}

	if len(m.stack) == 0 {
		return m.handleDashboardKey(msg)
	}
	return m.handleScreenKey(msg)
}

func (m *Model) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keymap.Down):
		if m.cursor < len(m.blocks)-1 {
			m.cursor++
		}
	case keyMatches(msg, m.keymap.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case keyMatches(msg, m.keymap.Top):
		m.cursor = 0
	case keyMatches(msg, m.keymap.Bottom):
		m.cursor = max(len(m.blocks)-1, 0)
	case keyMatches(msg, m.keymap.Enter):
		if m.cursor < len(m.blocks) {
			hash := m.blocks[m.cursor].Hash
			m.push(screen{kind: screenBlock, loading: true})
			return m, fetchBlockByHash(m.ctx, m.chain, hash)
		}
	case keyMatches(msg, m.keymap.Currency):
		m.cycleCurrency()
	}
	return m, nil
}

func (m *Model) handleScreenKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	top := &m.stack[len(m.stack)-1]
	switch top.kind {
	case screenBlock:
		switch {
		case keyMatches(msg, m.keymap.Down):
			if top.cursor < len(top.blockTxs)-1 {
				top.cursor++
			}
		case keyMatches(msg, m.keymap.Up):
			if top.cursor > 0 {
				top.cursor--
			}
		case keyMatches(msg, m.keymap.NextPg):
			return m, fetchBlockPage(m.ctx, m.chain, top.block.Hash, top.blockPage+1)
		case keyMatches(msg, m.keymap.PrevPg):
			if top.blockPage > 0 {
				return m, fetchBlockPage(m.ctx, m.chain, top.block.Hash, top.blockPage-1)
			}
		case keyMatches(msg, m.keymap.Enter):
			if top.cursor < len(top.blockTxs) {
				txid := top.blockTxs[top.cursor].TxID
				m.push(screen{kind: screenTx, loading: true})
				return m, fetchTx(m.ctx, m.chain, txid)
			}
		}
	case screenAddress:
		switch {
		case keyMatches(msg, m.keymap.Down):
			if top.cursor < len(top.addressTxs)-1 {
				top.cursor++
			}
		case keyMatches(msg, m.keymap.Up):
			if top.cursor > 0 {
				top.cursor--
			}
		case keyMatches(msg, m.keymap.Enter):
			if top.cursor < len(top.addressTxs) {
				txid := top.addressTxs[top.cursor].TxID
				m.push(screen{kind: screenTx, loading: true})
				return m, fetchTx(m.ctx, m.chain, txid)
			}
		}
	case screenTx:
		maxCursor := max(len(top.tx.Vin), len(top.tx.Vout)) - 1
		switch {
		case keyMatches(msg, m.keymap.Down):
			if top.cursor < maxCursor {
				top.cursor++
			}
		case keyMatches(msg, m.keymap.Up):
			if top.cursor > 0 {
				top.cursor--
			}
		}
	case screenWatchlist:
		total := len(top.watchlist) + len(top.watchlistWallets)
		switch {
		case keyMatches(msg, m.keymap.Down):
			if top.cursor < total-1 {
				top.cursor++
			}
		case keyMatches(msg, m.keymap.Up):
			if top.cursor > 0 {
				top.cursor--
			}
		case keyMatches(msg, m.keymap.Enter):
			if top.cursor < len(top.watchlist) {
				addr := top.watchlist[top.cursor].Address
				m.push(screen{kind: screenAddress, loading: true})
				return m, fetchAddress(m.ctx, m.chain, addr)
			} else if top.cursor < total {
				key := top.watchlistWallets[top.cursor-len(top.watchlist)].Key
				return m.openWallet(key)
			}
		case keyMatches(msg, m.keymap.Delete):
			if top.cursor < len(top.watchlist) && m.cfg.Watchlist != nil {
				addr := top.watchlist[top.cursor].Address
				if err := m.cfg.Watchlist.Remove(addr); err != nil {
					m.statusMsg = "unwatch: " + err.Error()
					return m, nil
				}
				m.statusMsg = "unwatched " + addr
				return m, m.refreshWatchlistScreen()
			} else if top.cursor < total && m.cfg.Watchlist != nil {
				key := top.watchlistWallets[top.cursor-len(top.watchlist)].Key
				if err := m.cfg.Watchlist.RemoveWallet(key); err != nil {
					m.statusMsg = "unwatch: " + err.Error()
					return m, nil
				}
				m.statusMsg = "unwatched wallet " + key
				return m, m.refreshWatchlistScreen()
			}
		}
	case screenLiveBlock:
		if keyMatches(msg, m.keymap.Filter) {
			top.liveBlockFilter = top.liveBlockFilter.Next()
		}
	case screenWallet:
		switch {
		case keyMatches(msg, m.keymap.Down):
			if top.cursor < len(top.wallet.Addresses)-1 {
				top.cursor++
			}
		case keyMatches(msg, m.keymap.Up):
			if top.cursor > 0 {
				top.cursor--
			}
		case keyMatches(msg, m.keymap.Enter):
			if top.cursor < len(top.wallet.Addresses) {
				addr := top.wallet.Addresses[top.cursor].Address
				m.push(screen{kind: screenAddress, loading: true})
				return m, fetchAddress(m.ctx, m.chain, addr)
			}
		}
	}
	return m, nil
}

func (m *Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		return m, nil
	case "enter":
		input := strings.TrimSpace(m.searchInput.Value())
		cmd, err := m.dispatchSearch(input)
		if err != nil {
			m.searchErr = err.Error()
			return m, nil
		}
		m.mode = modeNormal
		return m, cmd
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m *Model) handleCommandKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		return m, nil
	case "enter":
		input := strings.TrimSpace(m.commandInput.Value())
		m.mode = modeNormal
		return m.runCommand(input)
	}
	var cmd tea.Cmd
	m.commandInput, cmd = m.commandInput.Update(msg)
	return m, cmd
}

func (m *Model) runCommand(input string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return m, nil
	}
	switch fields[0] {
	case "q", "quit":
		m.quitting = true
		m.cancel()
		return m, tea.Quit
	case "block":
		if len(fields) < 2 {
			m.statusMsg = "usage: :block <height|hash|latest>"
			return m, nil
		}
		if fields[1] == "latest" || fields[1] == "current" || fields[1] == "tip" {
			if len(m.blocks) > 0 {
				hash := m.blocks[0].Hash
				m.push(screen{kind: screenBlock, loading: true})
				return m, fetchBlockByHash(m.ctx, m.chain, hash)
			}
			if m.tipHeight > 0 {
				m.push(screen{kind: screenBlock, loading: true})
				return m, fetchBlockByHeight(m.ctx, m.chain, m.tipHeight)
			}
			m.statusMsg = "tip height not known yet"
			return m, nil
		}
		return m.openByInput(fields[1])
	case "tx":
		if len(fields) < 2 {
			m.statusMsg = "usage: :tx <txid>"
			return m, nil
		}
		m.push(screen{kind: screenTx, loading: true})
		return m, fetchTx(m.ctx, m.chain, fields[1])
	case "addr":
		if len(fields) < 2 {
			m.statusMsg = "usage: :addr <address>"
			return m, nil
		}
		m.push(screen{kind: screenAddress, loading: true})
		return m, fetchAddress(m.ctx, m.chain, fields[1])
	case "wallet":
		if len(fields) < 2 {
			m.statusMsg = "usage: :wallet <xpub|ypub|zpub>"
			return m, nil
		}
		return m.openWallet(fields[1])
	case "provider":
		if len(fields) < 2 {
			m.statusMsg = "usage: :provider <name>|auto"
			return m, nil
		}
		if m.chainCtl == nil {
			m.statusMsg = "no fallback chain configured — single fixed provider"
			return m, nil
		}
		if fields[1] == "auto" {
			m.chainCtl.Unpin()
			m.statusMsg = "failover re-enabled"
			return m, nil
		}
		if err := m.chainCtl.Pin(fields[1]); err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}
		m.statusMsg = "pinned to " + m.chain.Name()
		return m, nil
	case "graph":
		if len(fields) < 2 {
			m.graphStyle = m.graphStyle.Next()
			m.statusMsg = "graph style: " + m.graphStyle.String()
			return m, nil
		}
		style, ok := components.ParseGraphStyle(fields[1])
		if !ok {
			m.statusMsg = "usage: :graph [braille|block|tty]"
			return m, nil
		}
		m.graphStyle = style
		m.statusMsg = "graph style: " + m.graphStyle.String()
		return m, nil
	case "watch":
		if len(fields) < 2 {
			m.statusMsg = "usage: :watch <address|xpub> [label]"
			return m, nil
		}
		if m.cfg.Watchlist == nil {
			m.statusMsg = "watchlist unavailable (no cache dir configured)"
			return m, nil
		}
		label := strings.Join(fields[2:], " ")
		if classify(fields[1]) == dispatchWallet {
			ww := cache.WatchedWallet{Key: fields[1], Label: label}
			if st, ok := m.resolvedWalletScriptTypes[fields[1]]; ok {
				ww.ScriptType = st.Name()
			}
			if err := m.cfg.Watchlist.AddWallet(ww); err != nil {
				m.statusMsg = "watch: " + err.Error()
				return m, nil
			}
			m.statusMsg = "watching wallet " + fields[1]
			return m, m.refreshWatchlistScreen()
		}
		if err := m.cfg.Watchlist.Add(cache.WatchedAddress{Address: fields[1], Label: label}); err != nil {
			m.statusMsg = "watch: " + err.Error()
			return m, nil
		}
		m.statusMsg = "watching " + fields[1]
		return m, m.refreshWatchlistScreen()
	case "unwatch":
		if len(fields) < 2 {
			m.statusMsg = "usage: :unwatch <address|xpub>"
			return m, nil
		}
		if m.cfg.Watchlist == nil {
			m.statusMsg = "watchlist unavailable (no cache dir configured)"
			return m, nil
		}
		if classify(fields[1]) == dispatchWallet {
			if err := m.cfg.Watchlist.RemoveWallet(fields[1]); err != nil {
				m.statusMsg = "unwatch: " + err.Error()
				return m, nil
			}
			m.statusMsg = "unwatched wallet " + fields[1]
			return m, m.refreshWatchlistScreen()
		}
		if err := m.cfg.Watchlist.Remove(fields[1]); err != nil {
			m.statusMsg = "unwatch: " + err.Error()
			return m, nil
		}
		m.statusMsg = "unwatched " + fields[1]
		return m, m.refreshWatchlistScreen()
	default:
		m.statusMsg = "unknown command: " + fields[0]
		return m, nil
	}
}

func (m *Model) dispatchSearch(input string) (tea.Cmd, error) {
	if input == "" {
		return nil, fmt.Errorf("empty search")
	}
	switch classify(input) {
	case dispatchHeight:
		h, _ := strconv.Atoi(input)
		m.push(screen{kind: screenBlock, loading: true})
		return fetchBlockByHeight(m.ctx, m.chain, h), nil
	case dispatchHash:
		m.push(screen{kind: screenTx, loading: true})
		return fetchAmbiguousHash(m.ctx, m.chain, input), nil
	case dispatchAddress:
		m.push(screen{kind: screenAddress, loading: true})
		return fetchAddress(m.ctx, m.chain, input), nil
	case dispatchWallet:
		_, cmd := m.openWallet(input)
		return cmd, nil
	default:
		return nil, fmt.Errorf("unrecognized input shape")
	}
}

func (m *Model) openByInput(input string) (tea.Model, tea.Cmd) {
	cmd, err := m.dispatchSearch(input)
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	return m, cmd
}

func (m *Model) push(s screen) { m.stack = append(m.stack, s) }
func (m *Model) pop() {
	if len(m.stack) > 0 {
		m.stack = m.stack[:len(m.stack)-1]
	}
}

func keyMatches(msg tea.KeyMsg, b interface{ Keys() []string }) bool {
	return slices.Contains(b.Keys(), msg.String())
}

// --- View ---

func (m *Model) View() string {
	if m.quitting {
		return ""
	}
	var body string
	switch {
	case len(m.stack) > 0:
		body = m.renderScreen(m.stack[len(m.stack)-1])
	default:
		body = m.renderDashboard()
	}

	if m.mode == modeSearch {
		hint := m.searchInput.View()
		if m.searchErr != "" {
			hint += "  " + lipgloss.NewStyle().Foreground(m.theme.Bad).Render(m.searchErr)
		}
		body = hint + "\n\n" + body
	}
	if m.mode == modeCommand {
		body = m.commandInput.View() + "\n\n" + body
	}
	if m.mode == modeWalletScriptType {
		body = m.renderWalletScriptTypePrompt() + "\n\n" + body
	}
	return body
}

func (m *Model) renderDashboard() string {
	return views.Dashboard(views.DashboardData{
		Units:           m.units,
		GraphStyle:      m.graphStyle,
		FPS:             m.fps.fps(),
		ProviderName:    m.chain.Name(),
		Degraded:        m.chainCtl != nil && m.chainCtl.Degraded(),
		Network:         m.cfg.Network,
		TipHeight:       m.tipHeight,
		LastBlockAt:     m.lastBlockTime(),
		HasPrice:        m.prices != nil && m.price.Value > 0,
		PriceConfigured: m.prices != nil,
		Price:           m.price,
		PriceHistory:    m.priceHistory,
		Fees:            m.fees,
		Mempool:         m.mempool,
		NextBlocks:      m.nextBlocks,
		Blocks:          m.blocks,
		Cursor:          m.cursor,
		Currency:        m.currentCurrency(),
		AssumedVSize:    m.cfg.AssumedVSize,
		Caps:            m.chain.Caps(),
		Stale:           m.staleMap(),
		Width:           m.width,
		Height:          m.height,
	}, m.theme)
}

func (m *Model) lastBlockTime() time.Time {
	if len(m.blocks) == 0 {
		return time.Time{}
	}
	return m.blocks[0].Time
}

func (m *Model) staleMap() map[string]time.Duration {
	out := map[string]time.Duration{}
	for stream, err := range m.lastErr {
		if err == nil {
			continue
		}
		panel := stream
		if stream == "blocks" {
			panel = "" // no dedicated panel; surfaced via header only
		}
		if panel == "" {
			continue
		}
		if good, ok := m.lastGood[stream]; ok {
			out[panel] = time.Since(good)
		} else {
			out[panel] = time.Hour // never succeeded; show as long-stale
		}
	}
	return out
}

// screenLoadingLabel names what's loading for views.Loading's "loading
// <label>…" text.
func screenLoadingLabel(kind screenKind) string {
	switch kind {
	case screenBlock:
		return "block"
	case screenTx:
		return "transaction"
	case screenAddress:
		return "address"
	case screenWallet:
		return "wallet"
	case screenLiveBlock:
		return "next block"
	default:
		return "data"
	}
}

func (m *Model) renderScreen(s screen) string {
	if s.err != nil {
		return lipgloss.NewStyle().Foreground(m.theme.Bad).Render("error: "+s.err.Error()) + "\n\n" + views.Help([]views.KeyHelp{{Keys: "esc", Action: "back"}}, m.theme)
	}
	// A screen's initial fetch hasn't resolved yet — an animated spinner
	// beats either a blank screen or the real view rendered on
	// zero-valued data (empty hash, "0 total tx", ...). Per-panel
	// progressive loads (the block treemap, address history) are a
	// separate concern handled inside their own view once this clears.
	if s.loading {
		return views.Loading(screenLoadingLabel(s.kind), m.theme)
	}
	switch s.kind {
	case screenBlock:
		return views.Block(views.BlockData{
			Block: s.block, Txs: s.blockTxs, Page: s.blockPage, Cursor: s.cursor,
			Caps: m.chain.Caps(), Width: m.width, Height: m.height,
			Units: m.units, Price: m.price.Value, Currency: m.currentCurrency(),
			TreemapTxs: s.blockTreemapTxs, TreemapTruncated: s.blockTreemapTruncated, TreemapLoading: s.blockTreemapLoading,
		}, m.theme)
	case screenTx:
		return views.Tx(views.TxData{
			Tx: s.tx, Cursor: s.cursor, Width: m.width, Height: m.height,
			Units: m.units, Price: m.price.Value, Currency: m.currentCurrency(),
		}, m.theme)
	case screenAddress:
		return views.Address(views.AddressData{
			Address: s.address, Txs: s.addressTxs, Cursor: s.cursor, Width: m.width, Height: m.height,
			Units: m.units, Price: m.price.Value, Currency: m.currentCurrency(), GraphStyle: m.graphStyle,
			History: s.addressHistory, HistoryTruncated: s.addressHistoryTruncated, HistoryLoading: s.addressHistoryLoading,
			Features: s.addressFeatures,
		}, m.theme)
	case screenHelp:
		return views.Help(m.keymap.Help(), m.theme)
	case screenConfig:
		return views.Config(m.configData(), m.theme)
	case screenWatchlist:
		entries := make([]views.WatchlistEntry, len(s.watchlist))
		for i, e := range s.watchlist {
			stats, loaded := s.watchlistAddrStats[e.Address]
			entries[i] = views.WatchlistEntry{Address: e.Address, Label: e.Label, AddedAt: e.AddedAt, Stats: stats, Loaded: loaded}
		}
		wallets := make([]views.WatchlistWalletEntry, len(s.watchlistWallets))
		for i, w := range s.watchlistWallets {
			wal, loaded := s.watchlistWalletStats[w.Key]
			wallets[i] = views.WatchlistWalletEntry{Key: w.Key, Label: w.Label, AddedAt: w.AddedAt, Wallet: wal, Loaded: loaded}
		}
		return views.Watchlist(views.WatchlistData{
			Entries: entries, Wallets: wallets, Cursor: s.cursor, Width: m.width, Height: m.height,
			Units: m.units, Price: m.price.Value, Currency: m.currentCurrency(),
		}, m.theme)
	case screenLiveBlock:
		return views.LiveBlock(views.LiveBlockData{
			Txs:       s.liveBlockTxs,
			Filter:    s.liveBlockFilter,
			Supported: m.chain.Caps().LiveBlockTxs,
			Width:     m.width,
			Height:    m.height,
		}, m.theme)
	case screenWallet:
		return views.Wallet(views.WalletData{
			Wallet: s.wallet, Cursor: s.cursor, Width: m.width, Height: m.height,
			Units: m.units, Price: m.price.Value, Currency: m.currentCurrency(),
		}, m.theme)
	}
	return ""
}

func (m *Model) configData() views.ConfigData {
	d := views.ConfigData{
		Network:    m.cfg.Network,
		Currency:   m.currentCurrency(),
		Theme:      m.theme.Name,
		GraphStyle: m.graphStyle,
		Caps:       m.chain.Caps(),
		TipHeight:  m.tipHeight,
	}
	if m.chainCtl == nil {
		d.Hosts = []views.ConfigHost{{Name: m.chain.Name(), Active: true}}
		return d
	}
	d.Pinned = m.chainCtl.Pinned()
	d.Degraded = m.chainCtl.Degraded()
	active := m.chain.Name()
	for _, name := range m.chainCtl.Hosts() {
		d.Hosts = append(d.Hosts, views.ConfigHost{Name: name, Active: name == active})
	}
	return d
}
