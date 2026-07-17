package model

import (
	"encoding/json"
	"time"
)

// TimeWindow is the half-open [Since, Until) range a HealthReport was computed
// over.
type TimeWindow struct {
	Since time.Time `json:"since"`
	Until time.Time `json:"until"`
}

// HealthReport answers `vd obs health` / GET /api/obs/health: tool-error counts
// and the top recurring error clusters over Window, compared against
// PrevWindow. Every number here is an investigate signal, not a verdict —
// agents fail-probe routinely, so `tool_spans.ok = false` on a grep no-match
// counts the same as a real fault.
type HealthReport struct {
	Window          TimeWindow       `json:"window"`
	PrevWindow      TimeWindow       `json:"prevwindow"`
	TotalErrors     int              `json:"totalerrors"`
	ErrorRate       float64          `json:"errorrate"`
	Delta           *int             `json:"delta"`
	ByTool          []ToolErrorStat  `json:"bytool"`
	ByAgent         []AgentErrorStat `json:"byagent"`
	Clusters        []ErrorCluster   `json:"clusters"`
	ErroredSessions int              `json:"erroredsessions"`
}

// MarshalJSON enforces the never-null rule for the report's list fields.
func (r HealthReport) MarshalJSON() ([]byte, error) {
	type report HealthReport
	v := report(r)
	if v.ByTool == nil {
		v.ByTool = []ToolErrorStat{}
	}
	if v.ByAgent == nil {
		v.ByAgent = []AgentErrorStat{}
	}
	if v.Clusters == nil {
		v.Clusters = []ErrorCluster{}
	}
	return json.Marshal(v)
}

// ToolErrorStat is one tool's error count within the window.
type ToolErrorStat struct {
	Tool  string `json:"tool"`
	Count int    `json:"count"`
}

// AgentErrorStat is one agent's error count within the window.
type AgentErrorStat struct {
	Agent string `json:"agent"`
	Count int    `json:"count"`
}

// EvidenceRef locates one turn precisely enough that `vd obs show <sessionid>
// --json` — which addresses turns by session + index — can fetch it; a bare
// turn id alone would not be.
type EvidenceRef struct {
	SessionID string `json:"sessionid"`
	TurnIndex int    `json:"turnindex"`
	TurnID    string `json:"turnid"`
}

// SkillRef is a skill name paired with its resolved file path, from
// inventory.Service — never guessed by obs itself.
type SkillRef struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// ErrorCluster groups errors sharing a normalized signature — the cluster's
// cross-run identity, stable across runs for the same underlying failure.
// CoOccurringSkills is a co-occurrence hint, never blame: SuggestedFocus is
// set only when the error text itself names a resolvable skill.
type ErrorCluster struct {
	Signature         string        `json:"signature"`
	Count             int           `json:"count"`
	LowSample         bool          `json:"lowsample"`
	Trend             string        `json:"trend"`
	AffectedTools     []string      `json:"affectedtools"`
	Sessions          []string      `json:"sessions"`
	CoOccurringSkills []SkillRef    `json:"cooccurringskills"`
	SuggestedFocus    *string       `json:"suggestedfocus"`
	Evidence          []EvidenceRef `json:"evidence"`
	Sample            string        `json:"sample"`
}

// MarshalJSON enforces the never-null rule for the cluster's list fields.
func (c ErrorCluster) MarshalJSON() ([]byte, error) {
	type cluster ErrorCluster
	v := cluster(c)
	if v.AffectedTools == nil {
		v.AffectedTools = []string{}
	}
	if v.Sessions == nil {
		v.Sessions = []string{}
	}
	if v.CoOccurringSkills == nil {
		v.CoOccurringSkills = []SkillRef{}
	}
	if v.Evidence == nil {
		v.Evidence = []EvidenceRef{}
	}
	return json.Marshal(v)
}

// HealthFilter scopes Health. Field names mirror the HTTP query params
// since/agent/project.
type HealthFilter struct {
	Since   time.Time `json:"since"`
	Agent   string    `json:"agent"`
	Project string    `json:"project"`
}
