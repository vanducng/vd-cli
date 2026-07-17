package obs

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
	"github.com/vanducng/vd-cli/v2/internal/obs/store"
)

// minSample is the "n < 3" trend guard from the goal: a cell or cluster below
// this count gets no trend arrow / delta — a 1->2 change reads as low sample,
// not +100%.
const minSample = 3

// Health computes the error-observability report: an investigate signal, never
// a verdict — agents fail-probe routinely (a grep no-match trips
// tool_spans.ok=false the same as a real fault), so a high count says "look
// here," not "this is broken."
func (s *Service) Health(ctx context.Context, f model.HealthFilter) (*model.HealthReport, error) {
	now := time.Now()
	// The store floors to whole milliseconds, so querying with the bare `now`
	// can exclude an error written in the same millisecond as this call (rare
	// in production's real write/query latency, but reliably hit by a test that
	// seeds then immediately queries). Pad the SQL upper bound by 1ms; the
	// reported Window.Until below stays the true, unpadded `now`.
	until := now.Add(time.Millisecond)

	events, err := s.st.ErrorEvents(ctx, f.Since, until, f.Agent, f.Project)
	if err != nil {
		return nil, err
	}
	total, err := s.st.ToolSpanTotal(ctx, f.Since, until, f.Agent, f.Project)
	if err != nil {
		return nil, err
	}

	rep := &model.HealthReport{
		Window:          model.TimeWindow{Since: f.Since, Until: now},
		TotalErrors:     len(events),
		ErrorRate:       errorRate(len(events), total),
		ByTool:          byTool(events),
		ByAgent:         byAgent(events),
		ErroredSessions: erroredSessions(events),
	}

	var prevEvents []store.ErrorEvent
	havePrev := !f.Since.IsZero()
	if havePrev {
		prevSince := f.Since.Add(-now.Sub(f.Since))
		if prevSince.After(f.Since) {
			prevSince = f.Since
		}
		prevEvents, err = s.st.ErrorEvents(ctx, prevSince, f.Since, f.Agent, f.Project)
		if err != nil {
			return nil, err
		}
		rep.PrevWindow = model.TimeWindow{Since: prevSince, Until: f.Since}
		if len(events) >= minSample && len(prevEvents) >= minSample {
			d := len(events) - len(prevEvents)
			rep.Delta = &d
		}
	}

	rep.Clusters = s.buildClusters(events, prevEvents, havePrev)
	return rep, nil
}

func errorRate(errored, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(errored) / float64(total)
}

func byTool(events []store.ErrorEvent) []model.ToolErrorStat {
	counts := map[string]int{}
	for _, e := range events {
		counts[e.ToolName]++
	}
	out := make([]model.ToolErrorStat, 0, len(counts))
	for name, n := range counts {
		out = append(out, model.ToolErrorStat{Tool: name, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tool < out[j].Tool
	})
	return out
}

func byAgent(events []store.ErrorEvent) []model.AgentErrorStat {
	counts := map[string]int{}
	for _, e := range events {
		counts[e.Agent]++
	}
	out := make([]model.AgentErrorStat, 0, len(counts))
	for name, n := range counts {
		out = append(out, model.AgentErrorStat{Agent: name, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Agent < out[j].Agent
	})
	return out
}

func erroredSessions(events []store.ErrorEvent) int {
	set := map[string]bool{}
	for _, e := range events {
		set[e.SessionID] = true
	}
	return len(set)
}

func (s *Service) buildClusters(events, prevEvents []store.ErrorEvent, havePrev bool) []model.ErrorCluster {
	groups := map[string][]store.ErrorEvent{}
	for _, e := range events {
		key := clusterKey(normalizeSignature(e.ErrorText))
		groups[key] = append(groups[key], e)
	}
	prevCounts := map[string]int{}
	for _, e := range prevEvents {
		prevCounts[clusterKey(normalizeSignature(e.ErrorText))]++
	}

	out := make([]model.ErrorCluster, 0, len(groups))
	for key, group := range groups {
		out = append(out, s.buildCluster(key, group, prevCounts[key], havePrev))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Signature < out[j].Signature
	})
	return out
}

func (s *Service) buildCluster(key string, group []store.ErrorEvent, prevCount int, havePrev bool) model.ErrorCluster {
	c := model.ErrorCluster{Signature: key, Count: len(group)}

	// LowSample guards the TREND comparison, not the cluster's own count: a
	// cluster with a large current Count still gets LowSample=true (and
	// Trend="low sample") when this signature's count in the PRIOR window is
	// below minSample — the baseline is too small for a percentage change to
	// mean anything, even though the current count is robust on its own. The
	// wire value stays "low sample" for API stability; the web view relabels
	// this chip "low baseline" for exactly that reason.
	c.LowSample = c.Count < minSample || (havePrev && prevCount < minSample)

	switch {
	case !havePrev:
		c.Trend = ""
	case c.LowSample:
		c.Trend = "low sample"
	case c.Count > prevCount:
		c.Trend = "up"
	case c.Count < prevCount:
		c.Trend = "down"
	default:
		c.Trend = "flat"
	}

	tools := map[string]bool{}
	sessions := map[string]bool{}
	skillNames := map[string]bool{}
	seenTurns := map[string]bool{}
	variantCounts := map[string]int{}
	for _, e := range group {
		tools[e.ToolName] = true
		sessions[e.SessionID] = true
		for _, sk := range e.Skills {
			skillNames[sk] = true
		}
		turnKey := e.TurnID
		if turnKey == "" {
			turnKey = e.SessionID + "#" + strconv.Itoa(e.TurnIndex)
		}
		if !seenTurns[turnKey] {
			seenTurns[turnKey] = true
			c.Evidence = append(c.Evidence, model.EvidenceRef{
				SessionID: e.SessionID, TurnIndex: e.TurnIndex, TurnID: e.TurnID,
			})
		}
		if c.Sample == "" && e.ErrorText != "" {
			c.Sample = e.ErrorText
		}
		// Full, pre-prefix-cut signature: this is what the prefix-key merge
		// collapsed, so it is what makes the merge verifiable rather than opaque.
		variantCounts[normalizeSignature(e.ErrorText)]++
	}
	c.AffectedTools = sortedKeys(tools)
	c.Sessions = sortedKeys(sessions)
	c.CoOccurringSkills = s.resolveSkills(sortedKeys(skillNames))
	c.SuggestedFocus = suggestFocus(c.CoOccurringSkills, group)
	c.Variants = topVariants(variantCounts, maxVariants)
	return c
}

// maxVariants caps how many distinct full signatures a cluster reports —
// enough to reveal a bad merge without turning the field into a full dump of
// every member.
const maxVariants = 5

// topVariants ranks distinct full signatures by count desc, name asc, and
// keeps the top limit. A cluster with only one distinct signature returns
// that one variant unchanged.
func topVariants(counts map[string]int, limit int) []model.ClusterVariant {
	out := make([]model.ClusterVariant, 0, len(counts))
	for sig, n := range counts {
		out = append(out, model.ClusterVariant{Signature: sig, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Signature < out[j].Signature
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// resolveSkills keeps only skills that resolve to an existing file on this
// machine: an unresolvable name is not evidence of anything, so it is dropped
// rather than surfaced with an empty path.
func (s *Service) resolveSkills(names []string) []model.SkillRef {
	if s.inv == nil {
		return nil
	}
	out := make([]model.SkillRef, 0, len(names))
	for _, name := range names {
		detail, err := s.inv.SkillDetail(name)
		if err != nil {
			continue
		}
		out = append(out, model.SkillRef{Name: name, Path: detail.Path})
	}
	return out
}

// suggestFocus is the hint-never-blame gate: a skill only becomes the
// suggested fix target when the error text itself names it — co-occurring in
// the same turn is never enough on its own.
func suggestFocus(skills []model.SkillRef, group []store.ErrorEvent) *string {
	for _, sk := range skills {
		for _, e := range group {
			if strings.Contains(e.ErrorText, sk.Name) {
				path := sk.Path
				return &path
			}
		}
	}
	return nil
}
