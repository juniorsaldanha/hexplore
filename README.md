# hexplore

A read-only Bitcoin block explorer for your terminal.

No wallet. No private keys. No signing. hexplore only ever reads public chain
data (via [mempool.space](https://mempool.space) and Esplora-compatible
APIs) — that's not a limitation, it's the point: a tool you can install and
run without having to audit it first.

- Live dashboard: price, fee market, mempool congestion, next-block
  projection, latest blocks
- Block, transaction, and address detail views with pagination and a
  live-fee-colored treemap of a block's real transactions
- Watch-only wallets: paste an xpub/ypub/zpub and hexplore derives and
  monitors every used address via the standard BIP32/44/49/84/86 gap-limit
  scan — still just reading public keys, never a private one
- A watchlist for addresses and wallets, rendered as a live tile dashboard
- Automatic failover across a fallback chain of hosts, with the active
  provider's capabilities always shown honestly (no fabricated values)

## Install

### macOS (Homebrew)

```sh
brew install --cask juniorsaldanha/tap/hexplore
```

### Linux

Pick whichever matches your distro:

```sh
# Debian / Ubuntu
curl -LO https://github.com/juniorsaldanha/hexplore/releases/latest/download/hexplore_linux_amd64.deb
sudo dpkg -i hexplore_linux_amd64.deb

# Fedora / RHEL
curl -LO https://github.com/juniorsaldanha/hexplore/releases/latest/download/hexplore_linux_amd64.rpm
sudo rpm -i hexplore_linux_amd64.rpm

# Arch Linux
curl -LO https://github.com/juniorsaldanha/hexplore/releases/latest/download/hexplore_linux_amd64.pkg.tar.xz
sudo pacman -U hexplore_linux_amd64.pkg.tar.xz

# Flatpak
flatpak install https://github.com/juniorsaldanha/hexplore/releases/latest/download/io.github.juniorsaldanha.hexplore.flatpak
flatpak run io.github.juniorsaldanha.hexplore

# Any distro — installer script (installs the plain binary to ~/.local/bin)
curl -fsSL https://raw.githubusercontent.com/juniorsaldanha/hexplore/main/install.sh | sh
```

Or grab a plain binary from the
[releases page](https://github.com/juniorsaldanha/hexplore/releases).

### Windows

Download `hexplore_windows_amd64.zip` (or `_arm64`) from the
[releases page](https://github.com/juniorsaldanha/hexplore/releases), unzip,
and run `hexplore.exe` from a terminal.

### From source

```sh
go install github.com/juniorsaldanha/hexplore/cmd/hexplore@latest
```

## Usage

```sh
hexplore              # launch the TUI
hexplore --json       # print one dashboard snapshot as JSON and exit
hexplore --version    # print the version
```

Config lives at `~/.config/hexplore/config.toml` (created on first run with
sane defaults — mempool.space, mainnet, no API key needed). See
[`docs/PLAN.md`](docs/PLAN.md) for the full config reference and
architecture notes.

### Key bindings

| Key | Action |
|---|---|
| `/` | search — height, hash, txid, address, or an xpub/ypub/zpub |
| `:` | command mode — `:watch`, `:wallet`, `:provider`, `:config`, `:q`, ... |
| `↵` | open selection |
| `Esc` / `Backspace` | back |
| `j`/`k`, `↑`/`↓` | move |
| `w` | watchlist |
| `u` | toggle units (BTC/sats/fiat) |
| `t` | cycle theme |
| `?` | help |
| `q` | quit |

Press `?` inside hexplore for the full, always-up-to-date list.

## License

MIT — see [LICENSE](LICENSE).
