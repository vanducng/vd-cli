package obs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vanducng/vd-cli/v2/internal/inventory"
	"github.com/vanducng/vd-cli/v2/internal/obs/model"
	"github.com/vanducng/vd-cli/v2/internal/obs/store"
)

func newTestService(t *testing.T) (*Service, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "obs.sqlite")
	repoRoot := t.TempDir()
	svc, err := NewService(dbPath, inventory.NewService(repoRoot, ""))
	if err != nil {
		t.Fatalf("open test service: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc, repoRoot
}

// writeTestSkill drops a minimal repo-managed skill so inventory.SkillDetail
// resolves it without needing skills.toml/skills.lock present.
func writeTestSkill(t *testing.T, repoRoot, name string) string {
	t.Helper()
	dir := filepath.Join(repoRoot, "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte("---\ndescription: test skill\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func seedError(t *testing.T, svc *Service, sessionID, agent, project, toolName, errText, skillName string, at time.Time) {
	t.Helper()
	turnID := sessionID + "-t1"
	rec := model.Record{
		Session: model.Session{ID: sessionID, Agent: agent, Project: project, StartedAt: at, EndedAt: at.Add(time.Minute)},
		Turns:   []model.Turn{{ID: turnID, SessionID: sessionID, Index: 0, StartedAt: at}},
		ToolSpans: []model.ToolSpan{
			{ID: sessionID + "-sp1", TurnID: turnID, Name: toolName, OK: false, Error: errText},
		},
	}
	if skillName != "" {
		rec.Skills = []model.Skill{{TurnID: turnID, Name: skillName}}
	}
	if err := svc.st.IngestFile(context.Background(), rec, store.Watermark{}); err != nil {
		t.Fatalf("seed error %s: %v", sessionID, err)
	}
}

// The goal's hint-never-blame gate: a skill invoked in the same turn as an
// error is listed as a co-occurrence hint either way, but SuggestedFocus only
// points at it when the error text itself names the skill.
func TestHealthHintNeverBlameInvariant(t *testing.T) {
	svc, repoRoot := newTestService(t)
	path := writeTestSkill(t, repoRoot, "debug")
	now := time.Now()

	seedError(t, svc, "sess-cooccur", model.AgentClaude, "vd-cli", "Bash",
		"open /repo/foo.go: no such file or directory", "debug", now)
	seedError(t, svc, "sess-named", model.AgentClaude, "vd-cli", "Bash",
		`skill "debug" failed: invalid argument`, "debug", now)

	rep, err := svc.Health(context.Background(), model.HealthFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Clusters) != 2 {
		t.Fatalf("clusters = %d, want 2: %+v", len(rep.Clusters), rep.Clusters)
	}

	var cooccur, named *model.ErrorCluster
	for i := range rep.Clusters {
		c := &rep.Clusters[i]
		for _, s := range c.Sessions {
			switch s {
			case "sess-cooccur":
				cooccur = c
			case "sess-named":
				named = c
			}
		}
	}
	if cooccur == nil || named == nil {
		t.Fatalf("did not find both clusters: %+v", rep.Clusters)
	}
	if cooccur.SuggestedFocus != nil {
		t.Fatalf("co-occurrence alone must not set suggested_focus, got %q", *cooccur.SuggestedFocus)
	}
	if len(cooccur.CoOccurringSkills) != 1 || cooccur.CoOccurringSkills[0].Path != path {
		t.Fatalf("co-occurring skill still expected as a hint: %+v", cooccur.CoOccurringSkills)
	}
	if named.SuggestedFocus == nil || *named.SuggestedFocus != path {
		t.Fatalf("error text naming the skill must set suggested_focus to %q, got %v", path, named.SuggestedFocus)
	}
}

// A small fixture over two windows: counts, the n<3 low-sample guard, and the
// per-cluster trend it drives.
func TestHealthCountsAndLowSample(t *testing.T) {
	svc, _ := newTestService(t)
	now := time.Now()
	since := now.Add(-1 * time.Hour)
	curTime := now.Add(-30 * time.Minute)
	prevTime := now.Add(-90 * time.Minute)

	for i := 0; i < 3; i++ {
		seedError(t, svc, fmt.Sprintf("sess-a-%d", i), model.AgentClaude, "vd-cli", "Bash",
			"open /repo/a.go: no such file or directory", "", curTime)
	}
	seedError(t, svc, "sess-b", model.AgentClaude, "vd-cli", "Read",
		"permission denied: /etc/shadow", "", curTime)
	for i := 0; i < 5; i++ {
		seedError(t, svc, fmt.Sprintf("sess-a-prev-%d", i), model.AgentClaude, "vd-cli", "Bash",
			"open /repo/other.go: no such file or directory", "", prevTime)
	}

	rep, err := svc.Health(context.Background(), model.HealthFilter{Since: since})
	if err != nil {
		t.Fatal(err)
	}
	if rep.TotalErrors != 4 {
		t.Fatalf("TotalErrors = %d, want 4", rep.TotalErrors)
	}
	if rep.Delta == nil || *rep.Delta != -1 {
		t.Fatalf("Delta = %v, want -1 (4 current vs 5 previous, both >= minSample)", rep.Delta)
	}
	if len(rep.Clusters) != 2 {
		t.Fatalf("clusters = %d, want 2: %+v", len(rep.Clusters), rep.Clusters)
	}

	var big, small *model.ErrorCluster
	for i := range rep.Clusters {
		c := &rep.Clusters[i]
		if c.Count == 3 {
			big = c
		} else if c.Count == 1 {
			small = c
		}
	}
	if big == nil || small == nil {
		t.Fatalf("expected one cluster of 3 and one of 1: %+v", rep.Clusters)
	}
	if big.LowSample {
		t.Fatalf("count=3 vs prev=5 must not be low sample: %+v", big)
	}
	if big.Trend != "down" {
		t.Fatalf("Trend = %q, want down (3 < prev 5)", big.Trend)
	}
	if !small.LowSample {
		t.Fatalf("count=1 must be low sample: %+v", small)
	}
	if small.Trend != "low sample" {
		t.Fatalf("Trend = %q, want %q", small.Trend, "low sample")
	}
}

// The reported bug: a hook-error family sharing a long identical preamble but
// varying past it (its own "Pattern:" line) fragmented into many clusters
// instead of ranking as the one dominant cluster it actually is. This proves
// the fix end to end through Health(), not just the clusterKey helper.
func TestHealthMergesSharedPrefixFamilyAcrossManyDistinctTails(t *testing.T) {
	svc, _ := newTestService(t)
	now := time.Now()
	preamble := strings.Repeat("NOTE this block is intentional and protects the context window. ", 3)

	for i := 0; i < 2; i++ {
		seedError(t, svc, fmt.Sprintf("sess-fam-a-%d", i), model.AgentClaude, "vd-cli", "Bash",
			preamble+"Pattern: alpha-specific-detail", "", now)
	}
	for i := 0; i < 3; i++ {
		seedError(t, svc, fmt.Sprintf("sess-fam-b-%d", i), model.AgentClaude, "vd-cli", "Bash",
			preamble+"Pattern: totally different wording entirely", "", now)
	}
	// An unrelated, much shorter error must not be swept into the family.
	seedError(t, svc, "sess-other", model.AgentClaude, "vd-cli", "Read",
		"File does not exist.", "", now)

	rep, err := svc.Health(context.Background(), model.HealthFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Clusters) != 2 {
		t.Fatalf("clusters = %d, want 2 (shared-prefix family merged, unrelated error separate): %+v",
			len(rep.Clusters), rep.Clusters)
	}

	top := rep.Clusters[0]
	if top.Count != 5 {
		t.Fatalf("merged family count = %d, want 5 (2+3, ranked above the 1-count unrelated cluster)", top.Count)
	}
	wantKey := string([]rune(preamble)[:clusterKeyPrefixLen])
	if top.Signature != wantKey {
		t.Fatalf("cluster Signature = %q, want the shared %d-char prefix %q", top.Signature, clusterKeyPrefixLen, wantKey)
	}
	if len(top.Sessions) != 5 {
		t.Fatalf("merged cluster Sessions = %d, want 5 distinct sessions", len(top.Sessions))
	}
}
