# hexplore — project plan (v2)

A terminal block explorer for Bitcoin. Read-only, provider-agnostic, no wallet.

> **v2 changes:** mempool.space is now the default provider, blockstream.info the
> fallback. Both run the same Esplora backend, so this is a hostname change, not a second
> adapter. Coinbase fee derivation drops from core to fallback-only. WebSocket moves
> forward. See §1.

---

## 0. The API landscape

blockstream.info's public API is free but rate-limited, with undocumented limits and paid
tiers for production use. Designing against a limit nobody publishes is not viable.

The important structural fact: **mempool.space runs Esplora (electrs) underneath and
serves the same endpoint shapes as blockstream.info.** They are drop-in interchangeable
for the core REST surface — the `esplora-client` library treats them as a single retry
pool. mempool.space then adds a `/api/v1/` namespace on top with everything Esplora lacks.

| Provider | Cost | Key | Core API | Extras |
|---|---|---|---|---|
| **mempool.space** (default) | free | none | Esplora-compatible | `/v1/` block fees, pool, exact next-block, WebSocket, prices |
| blockstream.info (fallback) | free tier, limited | optional | Esplora | none |
| Community instances | free | none | Esplora | `/v1/` (same software) |
| Self-hosted mempool.space | free | none | Esplora | `/v1/` + full privacy, no limits |

Community instances worth putting in the default rotation: `mempool.emzy.de`,
`mempool.bitaroo.net`. Same software, different operators — this is how you spread load
without asking the user to configure anything.

### What each requirement needs

| Requirement | mempool.space | Esplora fallback |
|---|---|---|
| Latest blocks | `/v1/blocks` with `extras` | `/blocks` |
| Height, hash, time, tx count, size, weight | native | native |
| Block fill % | derived: `weight / 4_000_000` | same |
| **Total fee collected** | `extras.totalFees` | derived via coinbase (§4) |
| **Average fee** | `extras.avgFeeRate` / `extras.medianFee` | derived |
| Mining pool | `extras.pool.name` | unavailable → renders `—` |
| **Next block projection** | `/v1/fees/mempool-blocks` (exact) | histogram approximation (§5) |
| Recommended fees | `/v1/fees/recommended` | `/fee-estimates` (lags, see §3) |
| Mempool congestion | `/mempool` | `/mempool` |
| Time since last block | derived from tip timestamp | same |
| BTC price + 24h | `/v1/prices` (no BRL) | none — needs `PriceProvider` |
| 24h sparkline | none | none — needs `PriceProvider` |
| Search | `/block-height/:h`, `/block/:hash`, `/tx/:txid`, `/address/:a` | same |
| Address autocomplete | unavailable | `/address-prefix/:prefix` |
| Live updates | WebSocket | polling only |

Two provider interfaces remain necessary: **no chain API returns a price sparkline**, and
mempool.space's price endpoint has no BRL. `PriceProvider` stays separate.

---

## 1. Architecture

```
cmd/hexplore/main.go
internal/
  app/          bubbletea root model, view routing, global keymap
  domain/       Block, Tx, Address, FeeTiers, MempoolState, Price, Capabilities
                — provider-neutral. Nothing here imports a provider.
  provider/
    provider.go     ChainProvider, PriceProvider, Capabilities, ErrUnsupported
    esplora/        the shared REST surface — mempool.space, blockstream.info, self-hosted
    mempoolspace/   embeds esplora, adds the /v1 endpoints and WebSocket
    chain/          fallback chain: ordered hosts, health tracking, failover
    price/
      coingecko/  binance/  kraken/  mempoolspace/
  enrich/       coinbase fee derivation, histogram projection, fill %, sat/vB → fiat
  cache/        bbolt (immutable) + memory TTL (volatile)
  poll/         refresh scheduler, backoff, rate budget, WebSocket supervisor
  config/       koanf TOML + go-keyring
  ui/
    theme/      lipgloss palette, fee-tier colours, themes
    components/ fillbar, sparkline, feegauge, blockcard, statusbar, spinner
    views/      dashboard, block, tx, address, search, config, help
```

**Dependency rule:** `ui` → `domain` ← `provider`. The UI never imports a provider package
and never sees a provider's JSON field names.

**`mempoolspace` embeds `esplora`** rather than duplicating it. It inherits every core
method and overrides only what `/v1/` improves. When you add a third Esplora-compatible
host later, it is a config entry, not a package.

### Interfaces

```go
type Capabilities struct {
    BlockFees      bool // totalFees/avgFee native, no derivation needed
    MiningPool     bool
    NextBlockExact bool // real projection vs histogram approximation
    AddressPrefix  bool // autocomplete search
    WebSocket      bool
    Testnet, Signet bool
}

type ChainProvider interface {
    Name() string
    Caps() Capabilities

    TipHeight(ctx) (int, error)
    LatestBlocks(ctx, n int) ([]domain.Block, error)
    BlockByHeight(ctx, h int) (domain.Block, error)
    BlockByHash(ctx, hash string) (domain.Block, error)
    BlockTxs(ctx, hash string, start int) ([]domain.Tx, error)

    Tx(ctx, txid string) (domain.Tx, error)
    Address(ctx, addr string) (domain.Address, error)
    AddressTxs(ctx, addr, lastSeen string) ([]domain.Tx, error)
    AddressPrefix(ctx, prefix string) ([]string, error) // may return ErrUnsupported

    Mempool(ctx) (domain.MempoolState, error)
    FeeEstimates(ctx) (domain.FeeTiers, error)
    NextBlocks(ctx, n int) ([]domain.ProjectedBlock, error)
}

type PriceProvider interface {
    Name() string
    Spot(ctx, currency string) (domain.Price, error)
    History24h(ctx, currency string) ([]float64, error)
    SupportedCurrencies() []string
}
```

`ErrUnsupported` is a sentinel, never a panic. A view receiving it renders `—` with a dim
reason (`pool: unavailable on this provider`), never a blank or a fabricated value.

---

## 2. The fallback chain

This is the part that makes the tool durable. Free public APIs change terms — plan for it.

```go
type Chain struct {
    hosts []ChainProvider   // ordered by preference
    state map[string]*health // consecutive failures, cooldown until
}
```

Behaviour:

1. Try hosts in order. On `429`, `5xx`, or timeout, mark the host unhealthy and advance.
2. Unhealthy hosts get an exponential cooldown capped at 5 minutes, then are retried.
3. `Caps()` returns the caps of the **currently active** host, so the UI degrades live when
   a failover moves from mempool.space to blockstream.info — the pool column empties, next
   block gains a `~`. It does not go stale or lie.
4. The status bar always names the active host. A user seeing `blockstream · degraded`
   understands instantly why the pool column emptied.
5. `:provider <name>` pins a host manually and disables failover for the session.

Default chain: `mempool.space` → `mempool.emzy.de` → `blockstream.info`.

The core REST surface is identical across all three, so failover is transparent for
everything except the `/v1/` extras.

---

## 3. Fee tiers

| Tier | mempool.space | Esplora | Label |
|---|---|---|---|
| High | `fastestFee` | `/fee-estimates` `"1"` | next block |
| Average | `halfHourFee` | `"3"` | ~30 min |
| Low | `hourFee` | `"6"` | ~1 hour |
| Economy | `economyFee` | `"144"` | ~1 day |

The two sources disagree during fee spikes and this is not a bug. Esplora's estimates come
from bitcoind's `estimatesmartfee`, which is deliberately conservative and slow to react;
mempool.space computes from the live mempool. Attribute the panel to the active provider
so the difference reads as provenance.

### The fiat column

A fee *rate* has no fiat value alone. It needs a transaction size:

```
fiat = sat_per_vb × assumed_vsize × price / 100_000_000
```

Default `assumed_vsize = 140` — a 1-in-2-out P2WPKH spend, the same assumption
mempool.space uses. **Print it in the panel border** (`per 140 vB tx`) so the number is
never mistaken for an absolute cost. Configurable.

---

## 4. Coinbase fee derivation (fallback path only)

Only runs when `!Caps().BlockFees`. mempool.space returns `extras.totalFees` directly.

Summing `/block/:hash/txs` would mean ~130 paginated requests for a 3,200-tx block. The
coinbase output is the shortcut — it equals subsidy plus all fees collected:

```
1. GET /block/:hash/txid/0     → coinbase txid       (immutable, cache forever)
2. GET /tx/:coinbase_txid      → sum(vout[].value)   (immutable, cache forever)
3. subsidy    = 50e8 >> (height / 210_000)  sats, 0 after 33 halvings
4. total_fees = sum(vout) − subsidy
5. avg_fee_rate = total_fees / (weight / 4)     sat/vB across the block
   avg_fee      = total_fees / (tx_count − 1)   sats per tx, excluding coinbase
```

Two requests per block instead of 130, both cacheable forever in bbolt keyed by block hash.

Edge cases: blocks 124724 and 501726 have provably-burned coinbase outputs where the miner
claimed less than the full subsidy, producing a negative "fee" — clamp at zero and flag.
Pre-210,000 heights work correctly with integer division.

Build this as a **pure function** over `(domain.Tx, height)` with table tests. It is the
highest-risk logic in the project and the easiest to verify in isolation.

---

## 5. Next-block projection

**Primary path** — `GET /v1/fees/mempool-blocks` returns projected blocks with
`blockSize`, `blockVSize`, `nTx`, `totalFees`, `medianFee`, `feeRange`. Render directly.

**Fallback path** — derive from `/mempool`'s `fee_histogram`, an array of
`[feerate, vsize]` descending, where each entry's vsize covers transactions paying more
than that feerate but less than the previous entry's:

```
acc = 0
for (feerate, vsize) in histogram:          # already descending
    if acc + vsize >= 1_000_000:            # one block ≈ 1M vB
        floor_rate = feerate
        break
    acc += vsize
projected_fill   = min(acc + vsize, 1_000_000) / 1_000_000
projected_median = vsize-weighted median of consumed buckets
```

Continue in 1M vB chunks for blocks 2, 3, 4.

Esplora's histogram has only ~7–10 buckets, so this is coarse. **Prefix every value from
this path with `~`** and never present it as a measurement. When `Caps().NextBlockExact`,
the prefix disappears.

---

## 6. Screens

### Dashboard

```
 hexplore                                  mempool.space · mainnet · ● 12s · 912,304

 ┌ BTC/USD ───────────────────┐┌ FEE MARKET ────────────────┐┌ MEMPOOL ──────────────────┐
 │  $64,812.40      ▲ +2.14%  ││  High    42 sat/vB   $3.81 ││  8,134 tx    3.44 vMB     │
 │  ▁▂▂▃▅▄▃▄▆▇▇▆▅▆▇█▇▆▅▆▇█▇▆  ││  Avg     28 sat/vB   $2.54 ││  ███████████░░░░░  3.4 blk│
 │  24h  L 63,100  H 65,240   ││  Low      6 sat/vB   $0.54 ││  total fee    0.292 BTC   │
 └────────────────────────────┘└ per 140 vB tx ─────────────┘└ congestion: moderate ─────┘

 ┌ NEXT BLOCK  ~4m ───────────┐┌ LATEST BLOCKS ──────────────────────────────────────────┐
 │   28 sat/vB    14 – 52     ││ 912,304    2m   ███████████████████░  98%   Foundry USA │
 │   1.42 MB      3,201 tx    ││            3,412 tx   1.63 MB   31 sat/vB   0.41 BTC    │
 │   0.0412 BTC fees          ││                                                          │
 │   ████████████████████ 96% ││ 912,303   14m   ████████████████████  99%   AntPool     │
 │                            ││            3,890 tx   1.71 MB   44 sat/vB   0.58 BTC    │
 │  +1   22 sat/vB  ████ 100% ││                                                          │
 │  +2   14 sat/vB  ████ 100% ││ 912,302   21m   ██████████████████░░  91%   ViaBTC      │
 └────────────────────────────┘└──────────────────────────────────────────────────────────┘
 / search   : command   ↵ open   r refresh   c config   ? help   q quit
```

Breakpoints, defined explicitly in `app` rather than left to lipgloss truncation:
- `< 100 cols` — the three top panels stack vertically
- `< 80 cols` — sparkline drops, block list loses its second detail line, pool column drops

### Other views

- **Block** — header fields, fee summary, pool, paginated tx list at 25/page (matching the
  API page size exactly, so pagination maps 1:1 onto requests).
- **Transaction** — inputs/outputs in two columns with values, fee and fee rate, size,
  weight, confirmation depth, RBF flag, spent/unspent per output via `/tx/:txid/outspends`.
  `↵` drills into any input or output address.
- **Address** — balance from `chain_stats.funded_txo_sum − spent_txo_sum`, mempool delta
  shown separately, history paged with `last_seen_txid`, UTXO list on `u`.
- **Search palette**, **Config**, **Help** (generated from the keymap struct so it cannot
  drift).

---

## 7. Search input detection

Dispatch on shape — no guessing round-trips:

| Pattern | Type |
|---|---|
| `^[0-9]{1,7}$` | height → `/block-height/:h` then `/block/:hash` |
| `^0{8}[0-9a-f]{56}$` | block hash (leading zeros from PoW) |
| `^[0-9a-f]{64}$` | txid → `/tx/:txid`, fall back to block hash on 404 |
| `^(1\|3)[a-km-zA-HJ-NP-Z1-9]{25,34}$` | base58 address |
| `^bc1[qp][a-z0-9]{38,58}$` | bech32 / bech32m address |
| `^[xyztuv](pub\|prv)[1-9A-HJ-NP-Za-km-z]{107}$` | extended key → watch-only wallet scan (see §14) |
| else, ≥ 3 chars | `/address-prefix/:prefix` if `Caps().AddressPrefix` |

Validate leading-zero count against current difficulty to disambiguate 64-hex block hash
vs txid, then fall back. Debounce prefix lookups at 250 ms.

---

## 8. Keybindings

k9s-inspired.

| Key | Action |
|---|---|
| `/` | search palette |
| `:` | command mode — `:block 912304`, `:tx <id>`, `:addr <a>`, `:wallet <xpub>`, `:watch`/`:unwatch <addr\|xpub>`, `:provider mempool`, `:config`, `:q` |
| `↵` | drill into selection |
| `Esc` / `Backspace` | pop view stack |
| `j k` `↑ ↓` | move · `g G` top/bottom · `Ctrl-d/u` half page |
| `Tab` `Shift-Tab` | cycle dashboard panel focus |
| `1`–`9` | jump to nth latest block |
| `r` | force refresh, bypassing cache |
| `y` | yank hash/txid/address to clipboard |
| `o` | open current object in the active provider's web explorer |
| `u` | toggle units — BTC / sats / fiat |
| `p` | cycle configured currencies |
| `t` | cycle theme · `c` config · `?` help · `q` quit |

Keep bindings in one `KeyMap` struct of `key.Binding` values so `?` renders from the same
source of truth.

---

## 9. Config

`~/.config/hexplore/config.toml`

```toml
[provider]
# Ordered fallback chain. All speak the Esplora REST surface, so failover is
# transparent for core data; /v1 extras degrade gracefully.
hosts = [
  "https://mempool.space/api",
  "https://mempool.emzy.de/api",
  "https://blockstream.info/api",
]
network  = "mainnet"        # mainnet | testnet | signet
timeout  = "10s"
failover = true             # false pins hosts[0]
websocket = true            # used when the active host supports it

[price]
provider   = "coingecko"    # coingecko | binance | kraken | mempoolspace | none
currency   = "USD"
currencies = ["USD", "BRL"] # cycled with `p`

[display]
theme            = "nord"
units            = "btc"    # btc | sat | fiat
blocks           = 10
assumed_tx_vsize = 140      # basis for the fee → fiat column
compact          = false

[refresh]                   # ignored for streams the WebSocket covers
tip     = "20s"
mempool = "30s"
fees    = "60s"
price   = "120s"

[cache]
path     = "~/.cache/hexplore"
max_size = "256MB"
```

**No API key in this file.** Keys live in the system keyring via `go-keyring`, service
`hexplore`, user `<host>-api-key` — same pattern as lazyado's PAT. The TOML stays
dotfile-safe and commit-safe. `HEXPLORE_API_KEY` is the headless fallback, with a warning.
If the keyring is unavailable (bare server, no D-Bus), say so plainly and require the env
var rather than silently writing plaintext.

Note that with mempool.space as default, **no key is needed at all** for normal use. The
key field exists for paid tiers and self-hosted auth, not as a prerequisite.

---

## 10. Refresh, caching, rate budget

Two cache tiers, because the data has two lifetimes:

- **Immutable** → bbolt on disk, no TTL, never invalidated: confirmed blocks, confirmed
  transactions, coinbase-derived fee totals. Keyed by hash. This is what makes a warm start
  instant.
- **Volatile** → memory with TTL: tip, mempool, fees, price.

A single `poll.Scheduler` goroutine owns all timers and WebSocket subscriptions and emits
`tea.Msg` into the Bubble Tea loop. Views never call providers directly — this is what
keeps the UI responsive and lets one 429 degrade one panel instead of freezing the app.

- When the active host has `Caps().WebSocket`, subscribe to new blocks and mempool stats
  and stand the pollers down. On disconnect, resume polling and reconnect with backoff.
- Per-endpoint exponential backoff on failure, capped at 5 min. Status bar shows
  `⚠ price stale 4m` rather than a stale number pretending to be live.
- Global token bucket per host. CoinGecko's free tier is the binding constraint at roughly
  5–15 calls/min; a 120 s price refresh sits comfortably inside it.
- New tip → fetch only the new block, prepend, drop the tail. Never re-fetch the list.

---

## 11. Details that make it feel good

- **Never block on the network.** First paint from cache with dim spinners on pending
  panels. Cold start shows the layout immediately, not an empty screen.
- **Sub-cell fill bars.** Use the eighth-block runes `▏▎▍▌▋▊▉█` for one-eighth of a column
  of precision. On a 20-cell bar that is 160 steps — the difference between a bar that
  looks designed and one that looks quantised.
- **Colour carries meaning, never decoration.** One green/amber/red ramp drives both fee
  tiers and mempool congestion so the panels read as one system. Honour `NO_COLOR`.
- **Age ticks locally.** "2m ago" recomputes every second from the cached timestamp with no
  network call. A clock that visibly runs is most of what makes a dashboard feel alive.
- **Every approximation is marked.** `~` on histogram-derived values, assumed vsize printed
  in the fee panel border. People will make fee decisions with this.
- **The status bar answers three questions at all times:** which host, how stale, what
  network. Testnet gets a loud accent — nobody should misread which chain they are on.

---

## 12. Roadmap

**Phase 1 — walking skeleton (v0.1)**
`domain` types, `ChainProvider` + esplora adapter, memory cache, dashboard with latest
blocks and fill bars, tip polling, search palette, block/tx/address views, help. No price,
no `/v1`. Goal: a working explorer against mempool.space.

**Phase 2 — the numbers (v0.2)**
`mempoolspace` adapter embedding esplora: `/v1/blocks` extras, `/v1/fees/recommended`,
`/v1/fees/mempool-blocks`, WebSocket. Fee market panel, mempool panel, next-block panel.
`PriceProvider` with CoinGecko + Binance, sparkline via `ntcharts/sparkline`, currency
switching. bbolt immutable cache.

**Phase 3 — resilience (v0.3)**
Fallback chain with health tracking and live capability switching. Coinbase fee derivation
for the Esplora path. TOML config + go-keyring, config screen, self-hosted base URLs,
testnet/signet.

**Phase 4 — polish (v0.4)**
Themes, responsive breakpoints, clipboard yank, open-in-browser, unit toggle, mouse support
via BubbleZone.

**Phase 5 — depth (v1.0)**
Address watchlist in bbolt, tx broadcast (`POST /tx`, pre-signed hex only), fee histogram
visualisation, difficulty adjustment countdown, halving countdown, Liquid support,
`--json` non-interactive mode.

### Stack

| | |
|---|---|
| Language | Go 1.23+ |
| TUI | `charmbracelet/bubbletea`, `lipgloss`, `bubbles` |
| Charts | `NimbleMarkets/ntcharts` — has a `sparkline` package built for Bubble Tea |
| Mouse | `lrstanley/bubblezone` (ntcharts requires it) |
| Storage | `go.etcd.io/bbolt` |
| Secrets | `zalando/go-keyring` |
| Config | `knadh/koanf` |
| HTTP | stdlib + `hashicorp/go-retryablehttp` |
| WebSocket | `coder/websocket` |

### Out of scope

No wallet. No private keys. No signing. hexplore is read-only, and saying so in the README
is a feature — it is the difference between a tool people install casually and one they
have to audit first. Phase 5 broadcast accepts pre-signed hex only.

---

## 13. Build order

1. `domain` types and the two interfaces. Get these right and the rest is mechanical.
2. `esplora` adapter: `TipHeight` + `LatestBlocks`, verified against
   `curl https://mempool.space/api/blocks`.
3. `enrich.CoinbaseFees` as a pure function with table tests, including blocks 124724 and
   501726. Highest-risk logic, easiest to test in isolation.
4. The fill bar component with eighth-block rendering.
5. The dashboard.. Then the dashboard.

---

## 14. Watch-only wallets (xpub/ypub/zpub)

`internal/wallet` derives addresses from an extended **public** key — no chain API knows
about HD wallets, so this is entirely client-side BIP32 math, and it never touches a
private key (an xprv/yprv/zprv is refused outright with `wallet.ErrPrivateKey`). This is
still "no wallet, no keys, no signing": deriving watch-only addresses from a public key
signs nothing and reveals nothing that key doesn't already reveal.

**Convention → script type**, via SLIP-132's version bytes (swapped to the standard
xpub/tpub bytes before handing the key to `hdkeychain`, the usual trick):

| Prefix | Convention | Script type |
|---|---|---|
| `zpub` / `vpub` (testnet) | BIP84 | native SegWit, P2WPKH |
| `ypub` / `upub` (testnet) | BIP49 | nested SegWit, P2SH-P2WPKH |
| `xpub` / `tpub` (testnet) | BIP44 **or** BIP86 | legacy P2PKH **or** Taproot P2TR |

A plain xpub/tpub is genuinely ambiguous — BIP44 and BIP86 use identical version bytes.
`wallet.AmbiguousScriptType` flags this; the app prompts the user to choose (a small
`modeWalletScriptType` overlay, j/k + enter) rather than silently guessing. ypub/zpub and
their testnet counterparts are convention-specific by construction and never prompt.

**Discovery** uses the standard gap-limit algorithm every compliant wallet already uses:
walk each of the two derivation chains (0 = receive/external, 1 = change/internal) in
batches of 20 (`walletGapLimit`), bounded-concurrency per batch like the block treemap's
tx sample; a chain is exhausted once a full batch comes back with zero activity. Capped at
`walletMaxScanPerChain` (2000) per chain — flagged `Truncated` rather than scanning
unbounded, same honesty pattern as the block treemap and address-history samples.

Verified against real BIP84/BIP49/BIP86 test vectors from the BIPs' own specs (not
self-derived) — the highest-value tests in the project after `enrich.CoinbaseFees`, for the
same reason: getting someone's own address wrong is a serious class of bug.

A watched wallet persists as a `cache.WatchedWallet` (a separate bbolt bucket from plain
addresses) — `:watch <xpub>` / `:unwatch <xpub>` auto-detect via the same shape dispatch as
search, and the watchlist screen lists wallets alongside addresses, opening into a scan on
Enter.
