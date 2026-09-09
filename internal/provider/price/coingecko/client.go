// Package coingecko implements domain's PriceProvider against CoinGecko's
// free API — the only provider with both broad fiat coverage (including
// BRL) and a 24h history series in one place (docs/PLAN.md §2).
package coingecko

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/juniorsaldanha/hexplore/internal/domain"
)

const DefaultBaseURL = "https://api.coingecko.com/api/v3"

// Not exhaustive — CoinGecko supports ~60 — just the currencies hexplore's
// config cycles through by default plus common majors.
var supportedCurrencies = []string{"USD", "BRL", "EUR", "GBP", "JPY", "CAD", "AUD", "CHF", "CNY"}

type Provider struct {
	BaseURL string
	HTTP    *http.Client
	APIKey  string // optional demo/paid key, sent as x-cg-demo-api-key
}

func New(baseURL string) *Provider {
	return &Provider{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *Provider) Name() string                  { return "coingecko" }
func (p *Provider) SupportedCurrencies() []string { return supportedCurrencies }

func (p *Provider) getJSON(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.BaseURL+path, nil)
	if err != nil {
		return err
	}
	if p.APIKey != "" {
		req.Header.Set("x-cg-demo-api-key", p.APIKey)
	}
	resp, err := p.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("coingecko: GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("coingecko: GET %s: read body: %w", path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("coingecko: GET %s: status %d: %s", path, resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("coingecko: GET %s: decode: %w", path, err)
	}
	return nil
}

func (p *Provider) Spot(ctx context.Context, currency string) (domain.Price, error) {
	cur := strings.ToLower(currency)
	var out map[string]map[string]float64
	path := fmt.Sprintf("/simple/price?ids=bitcoin&vs_currencies=%s&include_24hr_change=true", cur)
	if err := p.getJSON(ctx, path, &out); err != nil {
		return domain.Price{}, err
	}
	btc, ok := out["bitcoin"]
	if !ok {
		return domain.Price{}, fmt.Errorf("coingecko: no bitcoin price for currency %q", currency)
	}
	return domain.Price{
		Currency:  strings.ToUpper(currency),
		Value:     btc[cur],
		Change24h: btc[cur+"_24h_change"],
	}, nil
}

func (p *Provider) History24h(ctx context.Context, currency string) ([]float64, error) {
	cur := strings.ToLower(currency)
	var out struct {
		Prices [][2]float64 `json:"prices"`
	}
	path := fmt.Sprintf("/coins/bitcoin/market_chart?vs_currency=%s&days=1", cur)
	if err := p.getJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	series := make([]float64, len(out.Prices))
	for i, p := range out.Prices {
		series[i] = p[1]
	}
	return series, nil
}
