// Package pricing converts token counts to USD from a vendored subset of
// LiteLLM's table.
//
// It is the only cost path in the tree: session rows, `vd obs usage` totals and
// the HTTP API all bill through Table.Cost, so a rate fix lands everywhere at once.
package pricing

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
)

//go:embed prices.json
var embedded []byte

// Price is one model's per-token USD rates, keeping LiteLLM's field names so the
// vendored file stays diffable against upstream. A zero rate is not charged:
// OpenAI bills nothing to write its cache, so those entries carry no write rate.
type Price struct {
	Input      float64 `json:"input_cost_per_token"`
	Output     float64 `json:"output_cost_per_token"`
	CacheRead  float64 `json:"cache_read_input_token_cost"`
	CacheWrite float64 `json:"cache_creation_input_token_cost"`
}

// Table prices models. Load one and hold it on the service — a package-level
// table could not be overridden per test.
type Table struct {
	prices map[string]Price
}

// Load merges ~/.vd/obs/prices.json over the embedded table, per model, so a
// stale vendored rate is fixable without a rebuild. A missing override is normal.
func Load() (*Table, error) {
	prices := map[string]Price{}
	if err := json.Unmarshal(embedded, &prices); err != nil {
		return nil, fmt.Errorf("parse embedded prices: %w", err)
	}
	// The embedded table is self-sufficient. A missing home, an unreadable
	// override, or malformed override JSON must never fail Load — that would brick
	// desktop/web startup over an optional convenience file. Fall back to embedded.
	if path, err := overridePath(); err == nil {
		if b, err := os.ReadFile(path); err == nil {
			override := map[string]Price{}
			if err := json.Unmarshal(b, &override); err != nil {
				// malformed override: warn but keep the authoritative embedded table
				fmt.Fprintf(os.Stderr, "warning: ignoring malformed %s: %v\n", path, err)
			} else {
				for id, p := range override {
					prices[id] = p
				}
			}
		}
	}
	return &Table{prices: prices}, nil
}

func overridePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".vd", "obs", "prices.json"), nil
}

// Cost is what u bills at for the given model, and whether that model is priced
// at all. An unknown model reports false rather than 0, which would render as a
// free session instead of an unpriced one.
func (t *Table) Cost(id string, u model.TokenUsage) (float64, bool) {
	p, ok := t.lookup(id)
	if !ok {
		return 0, false
	}
	// ReasoningOutput is already inside Output; adding it bills thinking twice.
	return float64(u.Input)*p.Input +
		float64(u.Output)*p.Output +
		float64(u.CacheRead)*p.CacheRead +
		float64(u.CacheWrite)*p.CacheWrite, true
}

// lookup falls back to the longest matching prefix because ids carry date and
// variant suffixes: claude-sonnet-4-5-20250929 bills at the claude-sonnet-4-5
// rate, while gpt-5.6-terra must beat the shorter gpt-5.6.
// suffixOK accepts only a version/date tail after a prefix match. Without it,
// longest-prefix silently prices an unknown variant at its family's rate — a
// future gpt-5.6-nano would bill as gpt-5.6 and never appear in UnpricedModels,
// which is the "never guess a price" rule inverted.
var suffixOK = regexp.MustCompile(`^-(\d{4}-\d{2}-\d{2}|\d{6,8}|latest|preview|v\d+(\.\d+)*)$`)

func (t *Table) lookup(id string) (Price, bool) {
	if p, ok := t.prices[id]; ok {
		return p, true
	}
	best := ""
	for k := range t.prices {
		if len(k) > len(best) && strings.HasPrefix(id, k) && suffixOK.MatchString(id[len(k):]) {
			best = k
		}
	}
	if best == "" {
		return Price{}, false
	}
	return t.prices[best], true
}
