package app

import (
	"github.com/charmbracelet/bubbles/key"

	"github.com/juniorsaldanha/hexplore/internal/ui/views"
)

type KeyMap struct {
	Search    key.Binding
	Command   key.Binding
	Enter     key.Binding
	Back      key.Binding
	Up        key.Binding
	Down      key.Binding
	Top       key.Binding
	Bottom    key.Binding
	NextPg    key.Binding
	PrevPg    key.Binding
	Refresh   key.Binding
	HelpKey   key.Binding
	Config    key.Binding
	Yank      key.Binding
	Open      key.Binding
	Unit      key.Binding
	Currency  key.Binding
	Theme     key.Binding
	Watchlist key.Binding
	Delete    key.Binding
	LiveBlock key.Binding
	Filter    key.Binding
	Quit      key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Search:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Command:   key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command mode")),
		Enter:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "open selection")),
		Back:      key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("esc", "back")),
		Up:        key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "move up")),
		Down:      key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "move down")),
		Top:       key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
		Bottom:    key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		NextPg:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next page")),
		PrevPg:    key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev page")),
		Refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "force refresh")),
		HelpKey:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Config:    key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "config / provider status")),
		Yank:      key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yank hash/txid/address")),
		Open:      key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open in web explorer")),
		Unit:      key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "toggle units")),
		Currency:  key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "cycle currency (dashboard)")),
		Theme:     key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "cycle theme")),
		Watchlist: key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "watchlist")),
		Delete:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete (watchlist)")),
		LiveBlock: key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "live next-block graph")),
		Filter:    key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "cycle filter (live block)")),
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k KeyMap) Help() []views.KeyHelp {
	bindings := []key.Binding{
		k.Search, k.Command, k.Enter, k.Back,
		k.Up, k.Down, k.Top, k.Bottom,
		k.NextPg, k.PrevPg, k.Refresh, k.HelpKey, k.Config,
		k.Yank, k.Open, k.Unit, k.Currency, k.Theme, k.Watchlist, k.Delete, k.LiveBlock, k.Filter, k.Quit,
	}
	out := make([]views.KeyHelp, len(bindings))
	for i, b := range bindings {
		out[i] = views.KeyHelp{Keys: b.Help().Key, Action: b.Help().Desc}
	}
	return out
}
