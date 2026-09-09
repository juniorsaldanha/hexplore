package mempoolspace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/juniorsaldanha/hexplore/internal/domain"
)

// wsReadLimit is generous on purpose: a busy mempool's full projected-block
// snapshot can run several hundred KB in one message.
const wsReadLimit = 16 << 20

// wsEnvelope covers the handful of message shapes mempool.space's /v1/ws
// sends; only ProjectedBlockTransactions is used here. Reverse-engineered
// against the live feed and cross-checked against mempool/mempool's
// frontend source (frontend/src/app/shared/filters.utils.ts) for the flag
// bit meanings — this is not documented as a stable public API.
type wsEnvelope struct {
	ProjectedBlockTransactions *struct {
		Index             int     `json:"index"`
		Sequence          int64   `json:"sequence"`
		BlockTransactions [][]any `json:"blockTransactions"`
	} `json:"projected-block-transactions"`
}

// wsURL turns an Esplora-style base URL (https://mempool.space/api) into
// its WebSocket endpoint (wss://mempool.space/api/v1/ws).
func wsURL(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/v1/ws"
	return u.String(), nil
}

// fetchProjectedBlockTxs opens a short-lived WebSocket connection, asks for
// one not-yet-mined block's transaction list, and returns the first full
// snapshot — deliberately not a persistent streaming subscription. The
// panel that wants "live" data just calls this again on its own refresh
// timer, which is simpler than maintaining a long-lived connection with
// reconnect and delta-merging logic for data that doesn't need sub-second
// updates anyway.
func fetchProjectedBlockTxs(ctx context.Context, baseURL string, blockIndex int) ([]domain.ProjectedTx, error) {
	target, err := wsURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("mempoolspace: ws url: %w", err)
	}

	conn, _, err := websocket.Dial(ctx, target, nil)
	if err != nil {
		return nil, fmt.Errorf("mempoolspace: ws dial: %w", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(wsReadLimit)

	track := fmt.Sprintf(`{"track-mempool-block":%d}`, blockIndex)
	if err := conn.Write(ctx, websocket.MessageText, []byte(track)); err != nil {
		return nil, fmt.Errorf("mempoolspace: ws subscribe: %w", err)
	}

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return nil, fmt.Errorf("mempoolspace: ws read: %w", err)
		}
		var env wsEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue // not a message shape we care about
		}
		if env.ProjectedBlockTransactions == nil || env.ProjectedBlockTransactions.BlockTransactions == nil {
			continue
		}
		return decodeProjectedTxs(env.ProjectedBlockTransactions.BlockTransactions), nil
	}
}

// decodeProjectedTxs unpacks the compact [txid, fee, vsize, value, rate,
// flags, firstSeen] tuples. Field order confirmed empirically: fee/vsize
// divides out to exactly the given feerate.
func decodeProjectedTxs(raw [][]any) []domain.ProjectedTx {
	out := make([]domain.ProjectedTx, 0, len(raw))
	for _, entry := range raw {
		if len(entry) < 7 {
			continue
		}
		txid, ok := entry[0].(string)
		if !ok || txid == "" {
			continue
		}
		fee, _ := entry[1].(float64)
		vsize, _ := entry[2].(float64)
		value, _ := entry[3].(float64)
		rate, _ := entry[4].(float64)
		flags, _ := entry[5].(float64)
		firstSeen, _ := entry[6].(float64)
		out = append(out, domain.ProjectedTx{
			TxID:      txid,
			FeeSats:   int64(fee),
			VSize:     vsize,
			ValueSats: int64(value),
			FeeRate:   rate,
			Flags:     uint64(flags),
			FirstSeen: time.Unix(int64(firstSeen), 0),
		})
	}
	return out
}

func (p *Provider) ProjectedBlockTxs(ctx context.Context, blockIndex int) ([]domain.ProjectedTx, error) {
	return fetchProjectedBlockTxs(ctx, p.Provider.BaseURL, blockIndex)
}
