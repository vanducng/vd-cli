// Package obs is the shared seam for agent observability: one place where query
// and cost logic lives, consumed in-process by the vd obs CLI and serialized by
// the HTTP handlers. No frontend re-derives any of it.
//
// Unlike inventory.Service, which is a stateless struct of paths, Service owns a
// database handle — callers must Close it.
package obs

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/vanducng/vd-cli/v2/internal/obs/ingest"
	"github.com/vanducng/vd-cli/v2/internal/obs/model"
	"github.com/vanducng/vd-cli/v2/internal/obs/pricing"
	"github.com/vanducng/vd-cli/v2/internal/obs/store"
)

// syncDebounce keeps a page-refresh storm off the disk. It only helps vd web: a
// vd obs process starts cold every invocation and always syncs.
const syncDebounce = 5 * time.Second

// Service answers questions about local agent sessions.
type Service struct {
	st    *store.Store
	price *pricing.Table

	mu       sync.Mutex
	syncedAt time.Time
	syncing  bool
}

// NewService opens the cache at dbPath (empty for the default location).
func NewService(dbPath string) (*Service, error) {
	st, err := store.New(store.Config{Path: dbPath})
	if err != nil {
		return nil, err
	}
	tbl, err := pricing.Load()
	if err != nil {
		_ = st.Close()
		return nil, err
	}
	return &Service{st: st, price: tbl}, nil
}

// Close releases the database handle.
func (s *Service) Close() error { return s.st.Close() }

// Sync brings the cache up to date. It debounces rather than coalesces: the lock
// guards only the bookkeeping, never the multi-second ingest, and a caller that
// arrives while a sync is running (or within the debounce window) returns
// immediately with current data rather than waiting for the in-flight result.
// This keeps a fresh `vd web`'s parallel first-load requests from all blocking
// behind one 50s cold sync — the first caller pays it, the rest read what's there.
func (s *Service) Sync(ctx context.Context, opts ingest.SyncOptions) (ingest.SyncStats, error) {
	s.mu.Lock()
	if s.syncing || (!opts.Full && time.Since(s.syncedAt) < syncDebounce) {
		s.mu.Unlock()
		return ingest.SyncStats{}, nil
	}
	s.syncing = true
	s.mu.Unlock()

	// Deferred so a panic inside ingest (recovered per-request by net/http) can
	// never leave syncing stuck true and freeze obs for the process lifetime.
	var stats ingest.SyncStats
	var err error
	defer func() {
		s.mu.Lock()
		s.syncing = false
		if err == nil {
			s.syncedAt = time.Now()
		}
		s.mu.Unlock()
	}()
	stats, err = ingest.Sync(ctx, s.st, opts)
	return stats, err
}

// Sessions lists sessions with cost applied.
func (s *Service) Sessions(ctx context.Context, f model.SessionFilter) (*model.SessionList, error) {
	f.Limit = store.ClampLimit(f.Limit)
	rows, err := s.st.ListSessions(ctx, f)
	if err != nil {
		return nil, err
	}
	total, err := s.st.CountSessions(ctx, f)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		s.applyCost(&rows[i])
	}
	return &model.SessionList{Sessions: rows, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

// Session returns one session and a page of its turns.
func (s *Service) Session(ctx context.Context, idOrPrefix, agent string, turns, offset int) (*model.SessionDetail, error) {
	d, err := s.st.GetSession(ctx, idOrPrefix, agent, turns, offset)
	if err != nil {
		return nil, err
	}
	s.applyCost(&d.SessionSummary)

	// Resolve every subagent's model in one query rather than one-per-span: the
	// rollup must be priced at the subagent's own model, not the parent's.
	subIDs := map[string]bool{}
	for i := range d.Turns {
		for j := range d.Turns[i].ToolSpans {
			if sp := d.Turns[i].ToolSpans[j]; sp.RollupTokens != nil && sp.SubagentSessionID != "" {
				subIDs[sp.SubagentSessionID] = true
			}
		}
	}
	subModels := s.st.SessionModels(ctx, subIDs)

	for i := range d.Turns {
		if c, ok := s.price.Cost(d.Turns[i].Model, d.Turns[i].Tokens); ok {
			d.Turns[i].CostUSD = &c
		}
		for j := range d.Turns[i].ToolSpans {
			sp := &d.Turns[i].ToolSpans[j]
			if sp.RollupTokens == nil {
				continue
			}
			m := subModels[sp.SubagentSessionID]
			if m == "" {
				continue
			}
			if c, ok := s.price.Cost(m, *sp.RollupTokens); ok {
				sp.RollupCostUSD = &c
			}
		}
	}
	return d, nil
}

// Usage aggregates tokens and cost by day or month.
func (s *Service) Usage(ctx context.Context, f model.UsageFilter) (*model.UsageReport, error) {
	if f.Group == "" {
		f.Group = store.DefaultUsageGroup
	}
	raw, err := s.st.Usage(ctx, f)
	if err != nil {
		return nil, err
	}

	rep := &model.UsageReport{Rows: make([]model.UsageRow, 0, len(raw))}
	unpriced := map[string]bool{}
	var total float64
	priced := false

	for _, r := range raw {
		row := model.UsageRow{Date: r.Date, Agent: r.Agent, Model: r.Model, Tokens: r.Tokens}
		if c, ok := s.price.Cost(r.Model, r.Tokens); ok {
			row.CostUSD = &c
			total += c
			priced = true
		} else if r.Model != "" {
			unpriced[r.Model] = true
		}
		rep.Totals.Add(r.Tokens)
		rep.Rows = append(rep.Rows, row)
	}
	if priced {
		rep.TotalCostUSD = &total
	}
	rep.UnpricedModels = make([]string, 0, len(unpriced))
	for m := range unpriced {
		rep.UnpricedModels = append(rep.UnpricedModels, m)
	}
	sort.Strings(rep.UnpricedModels)
	return rep, nil
}

// Skills rolls up per-skill tool activity with per-invocation window attribution.
// It has no cost or enrichment of its own — the correctness lives in the store's
// window join — so this is a thin wrapper that packages the rows for both frontends.
func (s *Service) Skills(ctx context.Context, f model.SkillFilter) (*model.SkillReport, error) {
	rows, err := s.st.Skills(ctx, f)
	if err != nil {
		return nil, err
	}
	return &model.SkillReport{Skills: rows}, nil
}

// Hooks rolls up hook executions with block rates and co-occurring error share.
// Claude-only, like the underlying hook_execs.
func (s *Service) Hooks(ctx context.Context, f model.HookFilter) (*model.HookReport, error) {
	rows, err := s.st.Hooks(ctx, f)
	if err != nil {
		return nil, err
	}
	return &model.HookReport{Hooks: rows}, nil
}

// applyCost fills the money and cache-efficiency fields the store deliberately
// knows nothing about. A model with no price entry leaves CostUSD nil, which
// renders as "?" — never 0, which would read as free.
func (s *Service) applyCost(x *model.SessionSummary) {
	if c, ok := s.price.Cost(x.Model, x.Tokens); ok {
		x.CostUSD = &c
	}
	if hit, ok := cacheHitRate(x.Tokens); ok {
		x.CacheHitRate = &hit
	}
}

// cacheHitRate is cache reads over everything that could have been a cache read.
func cacheHitRate(t model.TokenUsage) (float64, bool) {
	den := t.CacheRead + t.CacheWrite + t.Input
	if den == 0 {
		return 0, false
	}
	return float64(t.CacheRead) / float64(den), true
}

// Err wraps a store sentinel so callers can map it without importing store.
var (
	ErrNotFound  = store.ErrSessionNotFound
	ErrAmbiguous = store.ErrAmbiguousPrefix
	ErrTooShort  = store.ErrPrefixTooShort
)
