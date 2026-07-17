package pricing

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
)

// loadIsolated points HOME at an empty dir so no test reads the developer's real
// ~/.vd/obs/prices.json and bills against whatever rates they happen to keep there.
func loadIsolated(t *testing.T) *Table {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	tbl, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return tbl
}

func assertCost(t *testing.T, got float64, ok bool, want float64) {
	t.Helper()
	if !ok {
		t.Fatalf("model is unpriced, want cost %v", want)
	}
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("cost = %.10f, want %.10f", got, want)
	}
}

func TestCostMatchesHandCalculation(t *testing.T) {
	tbl := loadIsolated(t)

	tests := []struct {
		name  string
		model string
		usage model.TokenUsage
		want  float64
	}{
		{
			// 1000*5e-06 + 2000*2.5e-05 + 100000*5e-07 + 5000*6.25e-06
			name:  "claude opus 4.8",
			model: "claude-opus-4-8",
			usage: model.TokenUsage{Input: 1000, Output: 2000, CacheRead: 100000, CacheWrite: 5000},
			want:  0.005 + 0.05 + 0.05 + 0.03125,
		},
		{
			// 2000*3e-06 + 1000*1.5e-05 + 200000*3e-07 + 8000*3.75e-06
			name:  "claude sonnet 4.5",
			model: "claude-sonnet-4-5",
			usage: model.TokenUsage{Input: 2000, Output: 1000, CacheRead: 200000, CacheWrite: 8000},
			want:  0.006 + 0.015 + 0.06 + 0.03,
		},
		{
			// 10000*1e-06 + 4000*5e-06 + 500000*1e-07 + 20000*1.25e-06
			name:  "claude haiku 4.5",
			model: "claude-haiku-4-5",
			usage: model.TokenUsage{Input: 10000, Output: 4000, CacheRead: 500000, CacheWrite: 20000},
			want:  0.01 + 0.02 + 0.05 + 0.025,
		},
		{
			// OpenAI bills nothing to write cache, so CacheWrite contributes 0:
			// 1000*5e-06 + 1000*3e-05 + 50000*5e-07
			name:  "gpt-5.6-sol (no cache-write charge)",
			model: "gpt-5.6-sol",
			usage: model.TokenUsage{Input: 1000, Output: 1000, CacheRead: 50000, CacheWrite: 4000},
			want:  0.005 + 0.03 + 0.025,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tbl.Cost(tt.model, tt.usage)
			assertCost(t, got, ok, tt.want)
		})
	}
}

// Cache reads dominate real Claude transcripts, so billing them at the input rate
// is the mistake worth guarding: it overstates this bundle by multiples.
func TestCacheReadIsNotBilledAtInputRate(t *testing.T) {
	tbl := loadIsolated(t)

	got, ok := tbl.Cost("claude-opus-4-8", model.TokenUsage{Input: 1000, CacheRead: 1000000})
	assertCost(t, got, ok, 0.005+0.5)

	if atInputRate := 1001000 * 5e-06; math.Abs(got-atInputRate) < 1e-9 {
		t.Errorf("cache reads billed at the input rate (%.4f)", atInputRate)
	}
}

func TestReasoningTokensAreNotDoubleBilled(t *testing.T) {
	tbl := loadIsolated(t)

	plain := model.TokenUsage{Input: 500, Output: 2000, CacheRead: 10000, CacheWrite: 1000}
	reasoning := plain
	reasoning.ReasoningOutput = 1500

	want, ok := tbl.Cost("gpt-5.6-terra", plain)
	if !ok {
		t.Fatal("gpt-5.6-terra unpriced")
	}
	got, ok := tbl.Cost("gpt-5.6-terra", reasoning)
	assertCost(t, got, ok, want)
}

func TestLongestPrefixResolvesVariantIDs(t *testing.T) {
	tbl := loadIsolated(t)

	usage := model.TokenUsage{Input: 1000, Output: 1000, CacheRead: 10000, CacheWrite: 1000}

	tests := []struct {
		name     string
		dated    string
		resolves string
	}{
		{"dated sonnet", "claude-sonnet-4-5-20250929", "claude-sonnet-4-5"},
		{"dated haiku", "claude-haiku-4-5-20251001", "claude-haiku-4-5"},
		// gpt-5.6 is also a prefix of gpt-5.6-terra and is 2x the rate; the longer
		// key has to win, so first-prefix-wins over map order fails here.
		{"variant beats shorter family key", "gpt-5.6-terra-2026-01-01", "gpt-5.6-terra"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, ok := tbl.Cost(tt.resolves, usage)
			if !ok {
				t.Fatalf("%s unpriced", tt.resolves)
			}
			got, ok := tbl.Cost(tt.dated, usage)
			assertCost(t, got, ok, want)
		})
	}
}

func TestUnknownModelIsUnpriced(t *testing.T) {
	tbl := loadIsolated(t)

	usage := model.TokenUsage{Input: 1000, Output: 1000, CacheRead: 10000, CacheWrite: 1000}

	// Every id here appears in the local corpus: a proxied model, Claude Code's
	// synthetic entries, and the bare aliases whose concrete model is unknowable.
	for _, id := range []string{"glm-5.2", "<synthetic>", "opus", "sonnet", "haiku", "opus[1m]", ""} {
		t.Run(id, func(t *testing.T) {
			got, ok := tbl.Cost(id, usage)
			if ok {
				t.Errorf("Cost(%q) reported priced at %v, want unpriced", id, got)
			}
			if got != 0 {
				t.Errorf("Cost(%q) = %v, want 0", id, got)
			}
		})
	}
}

func TestOverrideMergesOverEmbedded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".vd", "obs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `{
	  "claude-opus-4-8": {
	    "input_cost_per_token": 1e-05,
	    "output_cost_per_token": 5e-05,
	    "cache_read_input_token_cost": 1e-06,
	    "cache_creation_input_token_cost": 1.25e-05
	  },
	  "local-model-1": {
	    "input_cost_per_token": 2e-06,
	    "output_cost_per_token": 4e-06
	  }
	}`
	if err := os.WriteFile(filepath.Join(dir, "prices.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}

	tbl, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	usage := model.TokenUsage{Input: 1000, Output: 2000, CacheRead: 100000, CacheWrite: 5000}

	got, ok := tbl.Cost("claude-opus-4-8", usage)
	assertCost(t, got, ok, 0.01+0.1+0.1+0.0625)

	got, ok = tbl.Cost("local-model-1", model.TokenUsage{Input: 1000, Output: 1000})
	assertCost(t, got, ok, 0.002+0.004)

	// A per-model override must not drop the rest of the embedded table.
	got, ok = tbl.Cost("claude-sonnet-4-5", usage)
	assertCost(t, got, ok, 0.003+0.03+0.03+0.01875)
}

func TestMissingOverrideIsNotAnError(t *testing.T) {
	tbl := loadIsolated(t)

	if _, ok := tbl.Cost("claude-opus-4-8", model.TokenUsage{Input: 1}); !ok {
		t.Error("embedded table unusable without an override file")
	}
}

// Longest-prefix exists for dated ids; it must not price an unknown variant at its
// family's rate and report it as priced.
func TestUnknownVariantIsNotPricedByPrefix(t *testing.T) {
	tab, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"claude-sonnet-4-5-turbo", "gpt-5.6-nano"} {
		if _, ok := tab.Cost(id, model.TokenUsage{Input: 1000}); ok {
			t.Errorf("%s priced by prefix: an unknown variant must report unpriced, not guess its family's rate", id)
		}
	}
}
