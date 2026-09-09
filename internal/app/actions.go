package app

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/atotto/clipboard"
)

// currentTarget resolves what `y` and `o` act on: the selection under the
// cursor on the current screen, or the dashboard's selected block.
func (m *Model) currentTarget() (kind, id string, ok bool) {
	if len(m.stack) == 0 {
		if m.cursor < len(m.blocks) {
			return "block", m.blocks[m.cursor].Hash, true
		}
		return "", "", false
	}
	top := m.stack[len(m.stack)-1]
	switch top.kind {
	case screenBlock:
		if top.cursor < len(top.blockTxs) {
			return "tx", top.blockTxs[top.cursor].TxID, true
		}
		if top.block.Hash != "" {
			return "block", top.block.Hash, true
		}
	case screenTx:
		if top.tx.TxID != "" {
			return "tx", top.tx.TxID, true
		}
	case screenAddress:
		if top.cursor < len(top.addressTxs) {
			return "tx", top.addressTxs[top.cursor].TxID, true
		}
		if top.address.Address != "" {
			return "address", top.address.Address, true
		}
	}
	return "", "", false
}

func (m *Model) yank() {
	kind, id, ok := m.currentTarget()
	if !ok {
		m.statusMsg = "nothing selected to yank"
		return
	}
	if err := clipboard.WriteAll(id); err != nil {
		m.statusMsg = "clipboard unavailable: " + err.Error()
		return
	}
	m.statusMsg = fmt.Sprintf("yanked %s %s", kind, id)
}

func (m *Model) openInBrowser() {
	kind, id, ok := m.currentTarget()
	if !ok {
		m.statusMsg = "nothing selected to open"
		return
	}
	url := fmt.Sprintf("https://%s/%s/%s", m.chain.Name(), kind, id)
	if err := openURL(url); err != nil {
		m.statusMsg = "could not open browser: " + err.Error()
		return
	}
	m.statusMsg = "opened " + url
}

func openURL(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
