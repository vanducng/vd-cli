// Package model is the wire contract for vd obs: the entities parsed out of local
// agent transcripts and the DTOs the CLI and HTTP API both render.
//
// Every field carries an explicit flat-lowercase json tag, matching
// internal/inventory/types.go and web/src/types.ts. Untagged fields marshal as
// Go identifiers (CostUSD, CacheHitRate), which forces every frontend to guess.
package model

import (
	"encoding/json"
	"time"
)

// Agent identifies which coding agent produced a session.
const (
	AgentClaude = "claude-code"
	AgentCodex  = "codex"
)

// TokenUsage is one accounting of billed tokens. ReasoningOutput is already
// counted inside Output — billing it separately double-charges.
type TokenUsage struct {
	Input           int `json:"input"`
	Output          int `json:"output"`
	CacheRead       int `json:"cacheread"`
	CacheWrite      int `json:"cachewrite"`
	ReasoningOutput int `json:"reasoningoutput"`
}

// Add accumulates u into t.
func (t *TokenUsage) Add(u TokenUsage) {
	t.Input += u.Input
	t.Output += u.Output
	t.CacheRead += u.CacheRead
	t.CacheWrite += u.CacheWrite
	t.ReasoningOutput += u.ReasoningOutput
}

// Total is every token the session consumed, cache included.
func (t TokenUsage) Total() int {
	return t.Input + t.Output + t.CacheRead + t.CacheWrite
}

// Session is one top-level agent conversation. Subagent transcripts are sessions
// too, linked by ParentID; they contribute usage but are not listed on their own.
type Session struct {
	ID    string `json:"id"`
	Agent string `json:"agent"`
	Title string `json:"title"`
	// TitleDerived is true when Title was inferred from the first user prompt
	// (Codex carries no title field of its own) rather than sourced from the
	// transcript (e.g. Claude's ai-title record).
	TitleDerived bool      `json:"titlederived"`
	CWD          string    `json:"cwd"`
	Project      string    `json:"project"`
	GitBranch    string    `json:"gitbranch"`
	GitSHA       string    `json:"gitsha"`
	Model        string    `json:"model"`
	CLIVersion   string    `json:"cliversion"`
	Originator   string    `json:"originator"`
	WorkflowID   string    `json:"workflowid,omitempty"`
	ParentID     string    `json:"parentid,omitempty"`
	StartedAt    time.Time `json:"startedat"`
	EndedAt      time.Time `json:"endedat"`
}

// SessionSummary is one row of `vd obs sessions` / GET /api/obs/sessions.
// CostUSD is nil when the model has no price entry — never 0, which reads as free.
// Tokens covers this session's own turns only; a parent does not absorb its
// subagents' usage, so summing sessions[].tokens is less than `vd obs usage`.
type SessionSummary struct {
	Session
	TurnCount    int        `json:"turncount"`
	Tokens       TokenUsage `json:"tokens"`
	CostUSD      *float64   `json:"costusd"`
	CacheHitRate *float64   `json:"cachehitrate"`
}

// SessionList is the named-collection envelope for list endpoints, matching the
// existing {"hooks": ...} shape. Total is the unpaginated count; Limit is the
// clamped value actually applied, not what the caller asked for.
type SessionList struct {
	Sessions []SessionSummary `json:"sessions"`
	Total    int              `json:"total"`
	Limit    int              `json:"limit"`
	Offset   int              `json:"offset"`
}

// SessionDetail is one session with a page of its turns.
type SessionDetail struct {
	SessionSummary
	Turns []Turn `json:"turns"`
}

// MarshalJSON enforces the never-null rule for the turn list.
func (d SessionDetail) MarshalJSON() ([]byte, error) {
	type detail SessionDetail
	v := detail(d)
	if v.Turns == nil {
		v.Turns = []Turn{}
	}
	return json.Marshal(v)
}

// MarshalJSON enforces the never-null rule for the session list.
func (l SessionList) MarshalJSON() ([]byte, error) {
	type list SessionList
	v := list(l)
	if v.Sessions == nil {
		v.Sessions = []SessionSummary{}
	}
	return json.Marshal(v)
}

// MarshalJSON enforces the never-null rule for usage rows and unpriced models.
func (r UsageReport) MarshalJSON() ([]byte, error) {
	type report UsageReport
	v := report(r)
	if v.Rows == nil {
		v.Rows = []UsageRow{}
	}
	if v.UnpricedModels == nil {
		v.UnpricedModels = []string{}
	}
	return json.Marshal(v)
}

// Turn is one user prompt and everything the agent did in response.
type Turn struct {
	ID           string     `json:"id"`
	SessionID    string     `json:"sessionid"`
	Index        int        `json:"index"`
	Model        string     `json:"model"`
	PromptText   string     `json:"prompttext"`
	ResponseText string     `json:"responsetext"`
	StartedAt    time.Time  `json:"startedat"`
	DurationMs   int64      `json:"durationms"`
	Tokens       TokenUsage `json:"tokens"`
	CostUSD      *float64   `json:"costusd"`
	ToolSpans    []ToolSpan `json:"toolspans"`
	HookExecs    []HookExec `json:"hookexecs"`
	Skills       []Skill    `json:"skills"`
}

// MarshalJSON keeps the frozen contract's "lists are arrays, never null" rule a
// property of the type rather than of every caller that builds a Turn by hand.
func (t Turn) MarshalJSON() ([]byte, error) {
	type turn Turn // shed the method to avoid recursing
	v := turn(t)
	if v.ToolSpans == nil {
		v.ToolSpans = []ToolSpan{}
	}
	if v.HookExecs == nil {
		v.HookExecs = []HookExec{}
	}
	if v.Skills == nil {
		v.Skills = []Skill{}
	}
	return json.Marshal(v)
}

// ToolSpan is one tool invocation inside a turn. Output/Error carry the payloads
// the transcript view renders; both are truncated to MaxPayloadBytes at ingest.
type ToolSpan struct {
	ID         string `json:"id"`
	TurnID     string `json:"turnid"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Input      string `json:"input"`
	Output     string `json:"output"`
	Error      string `json:"error"`
	DurationMs int64  `json:"durationms"`
	OK         bool   `json:"ok"`

	// Subagent rollups are display-only. Their tokens are counted from the
	// subagent's own transcript; adding these too would double-count.
	// Pointer, because encoding/json never omits a zero struct.
	SubagentSessionID string      `json:"subagentsessionid,omitempty"`
	SubagentName      string      `json:"subagentname,omitempty"`
	RollupTokens      *TokenUsage `json:"rolluptokens,omitempty"`
	RollupCostUSD     *float64    `json:"rollupcostusd,omitempty"`
}

// HookExec is one hook run. Claude Code only — Codex emits no hook events.
type HookExec struct {
	TurnID     string `json:"turnid"`
	HookName   string `json:"hookname"`
	Event      string `json:"event"`
	Seq        int    `json:"seq"`
	DurationMs int64  `json:"durationms"`
	ExitCode   int    `json:"exitcode"`
}

// Skill is one skill invocation. Claude Code only.
type Skill struct {
	TurnID string `json:"turnid"`
	Name   string `json:"name"`
	Seq    int    `json:"seq"`
	Args   string `json:"args"`
}

// SkillSummary is one row of `vd obs skills`: a skill (or the "(none)" bucket for
// unattributed activity) with the tool work charged to its invocation windows. A
// window opens at an invocation's turn and closes at the next invocation in that
// session, or session end; spans and turns inside it attribute to that skill.
// Counting by session broadcast instead overcounts ~4.7x (measured) — never do it.
type SkillSummary struct {
	Name         string   `json:"name"`
	Agents       []string `json:"agents"`
	Invocations  int      `json:"invocations"`
	Sessions     int      `json:"sessions"`
	SoloSessions int      `json:"solosessions"`
	ToolCalls    int      `json:"toolcalls"`
	ToolErrors   int      `json:"toolerrors"`
	ErrRate      *float64 `json:"errrate"`
	Tokens       int      `json:"tokens"`
	// Correctness proxies, classified at query time (no raw text is ever exposed).
	// Corrections counts user turns opening with a correction phrase; Aborts counts
	// turns carrying the interrupt marker. Counters flag candidates — only reading
	// the transcript proves fault.
	Corrections int `json:"corrections"`
	Aborts      int `json:"aborts"`
}

// SkillNone is the bucket name for tool activity that precedes any invocation or
// happens in a session that invoked no skill. It sorts last in a report.
const SkillNone = "(none)"

// SkillReport is the whole `vd obs skills` answer, sorted errors-desc with the
// "(none)" bucket forced last.
type SkillReport struct {
	Skills []SkillSummary `json:"skills"`
}

// MarshalJSON enforces the never-null rule for the skills list.
func (r SkillReport) MarshalJSON() ([]byte, error) {
	type report SkillReport
	v := report(r)
	if v.Skills == nil {
		v.Skills = []SkillSummary{}
	}
	return json.Marshal(v)
}

// MarshalJSON keeps a skill's agents an array, never null.
func (s SkillSummary) MarshalJSON() ([]byte, error) {
	type summary SkillSummary
	v := summary(s)
	if v.Agents == nil {
		v.Agents = []string{}
	}
	return json.Marshal(v)
}

// SkillFilter scopes a skills rollup. Fields mirror the HTTP query params: agent,
// project, since. All three are session-level, so a session is wholly in or out.
type SkillFilter struct {
	Agent   string    `json:"agent"`
	Project string    `json:"project"`
	Since   time.Time `json:"since"`
}

// HookSummary is one row of `vd obs hooks`: a hook/event pair with how often it
// fired, how often it exited nonzero, and — for gate-class hooks — the share of
// same-turn tool errors that co-occur with its blocks. Claude Code only; Codex
// emits no hook events.
type HookSummary struct {
	HookName     string `json:"hookname"`
	Event        string `json:"event"`
	Fires        int    `json:"fires"`
	NonzeroExits int    `json:"nonzeroexits"`
	// BlockRate and ErrShare stay nil when undefined (no fires, no errors in scope)
	// rather than 0, so "never fired" never reads as "0% block rate".
	BlockRate *float64 `json:"blockrate"`
	ErrShare  *float64 `json:"errshare"`
}

// HookReport is the whole `vd obs hooks` answer, sorted by nonzero exits desc.
type HookReport struct {
	Hooks []HookSummary `json:"hooks"`
}

// MarshalJSON enforces the never-null rule for the hooks list.
func (r HookReport) MarshalJSON() ([]byte, error) {
	type report HookReport
	v := report(r)
	if v.Hooks == nil {
		v.Hooks = []HookSummary{}
	}
	return json.Marshal(v)
}

// HookFilter scopes a hook rollup by the same session-level dimensions as skills.
type HookFilter struct {
	Agent   string    `json:"agent"`
	Project string    `json:"project"`
	Since   time.Time `json:"since"`
}

// UsageRow is one grouped bucket of `vd obs usage`.
type UsageRow struct {
	Date    string     `json:"date"`
	Agent   string     `json:"agent"`
	Model   string     `json:"model"`
	Tokens  TokenUsage `json:"tokens"`
	CostUSD *float64   `json:"costusd"`
}

// UsageReport is the whole `vd obs usage` answer. UnpricedModels names every model
// that had no price entry, so a low total is never silently mistaken for a cheap one.
type UsageReport struct {
	Rows           []UsageRow `json:"rows"`
	Totals         TokenUsage `json:"totals"`
	TotalCostUSD   *float64   `json:"totalcostusd"`
	UnpricedModels []string   `json:"unpricedmodels"`
}

// SessionFilter selects and pages sessions. Field names mirror the HTTP query
// params 1:1: agent, project, q, since, limit, offset, sort.
type SessionFilter struct {
	Agent   string    `json:"agent"`
	Project string    `json:"project"`
	Q       string    `json:"q"`
	Since   time.Time `json:"since"`
	Limit   int       `json:"limit"`
	Offset  int       `json:"offset"`
	Sort    string    `json:"sort"`
}

// UsageFilter groups usage by day or month.
type UsageFilter struct {
	Group string    `json:"group"`
	Agent string    `json:"agent"`
	Since time.Time `json:"since"`
}

// Record is one parsed transcript file, handed from ingest to the store as a
// single unit of work.
type Record struct {
	Session   Session    `json:"session"`
	Turns     []Turn     `json:"turns"`
	ToolSpans []ToolSpan `json:"toolspans"`
	HookExecs []HookExec `json:"hookexecs"`
	Skills    []Skill    `json:"skills"`
}
