package ingest

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
)

// CodexSessionsPath returns the absolute path to ~/.codex/sessions.
func CodexSessionsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".codex", "sessions"), nil
}

// EnumerateCodex returns every rollout transcript under root's YYYY/MM/DD tree,
// in lexical order (which is chronological, given the date path and timestamped
// filenames).
func EnumerateCodex(root string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(root, "*", "*", "*", "rollout-*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("glob codex rollouts under %s: %w", root, err)
	}
	return paths, nil
}

// ParseCodex reads a whole rollout from r into a Record, returning the offset of
// the last complete line. r must be positioned at the start of the file: a
// mid-file resume would re-emit a turn carrying only post-offset tokens, and the
// store's upsert replaces rather than merges, so stored totals would shrink.
// Sync skips unchanged files instead.
func ParseCodex(r io.Reader, st *ScanState) (model.Record, int64, error) {
	p := &codexParser{
		st:     st,
		byID:   map[string]*codexTurn{},
		calls:  map[string]*model.ToolSpan{},
		callAt: map[string]time.Time{},
	}
	off, oversized, err := ScanLines(r, 0, p.line)
	for i := 0; i < oversized; i++ {
		p.st.NoteUnknown("oversized_line")
	}
	st.Offset = off
	rec := p.record()
	namespaceTurnIDs(&rec)
	return rec, off, err
}

type codexTokens struct {
	Input     int `json:"input_tokens"`
	Cached    int `json:"cached_input_tokens"`
	Output    int `json:"output_tokens"`
	Reasoning int `json:"reasoning_output_tokens"`
}

type codexRecord struct {
	Timestamp time.Time    `json:"timestamp"`
	Type      string       `json:"type"`
	Payload   codexPayload `json:"payload"`
}

// codexPayload is the union of every rollout payload we model. The rollout keys
// do not collide across record types, so one struct beats a decode per type.
type codexPayload struct {
	Type string `json:"type"`

	SessionID  string          `json:"session_id"`
	ID         string          `json:"id"`
	CWD        string          `json:"cwd"`
	Originator string          `json:"originator"`
	CLIVersion string          `json:"cli_version"`
	Source     json.RawMessage `json:"source"`
	Git        struct {
		Branch     string `json:"branch"`
		CommitHash string `json:"commit_hash"`
	} `json:"git"`

	TurnID string `json:"turn_id"`
	Model  string `json:"model"`

	Message string `json:"message"`

	Info struct {
		Last  *codexTokens `json:"last_token_usage"`
		Total *codexTokens `json:"total_token_usage"`
	} `json:"info"`

	DurationMs       int64  `json:"duration_ms"`
	LastAgentMessage string `json:"last_agent_message"`

	CallID    string          `json:"call_id"`
	Name      string          `json:"name"`
	Status    string          `json:"status"`
	Input     string          `json:"input"`
	Arguments string          `json:"arguments"`
	Output    json.RawMessage `json:"output"`
	Meta      struct {
		TurnID string `json:"turn_id"`
	} `json:"internal_chat_message_metadata_passthrough"`
}

type codexTurn struct {
	id       string
	model    string
	started  time.Time
	prompts  []string
	response string
	duration int64
	tokens   model.TokenUsage
	explicit bool
}

type codexParser struct {
	st         *ScanState
	sess       model.Session
	turns      []*codexTurn
	byID       map[string]*codexTurn
	cur        *codexTurn
	spans      []*model.ToolSpan
	calls      map[string]*model.ToolSpan
	callAt     map[string]time.Time
	last       time.Time
	lastTokens model.TokenUsage
	lastTotal  model.TokenUsage
	sawTokens  bool
}

func (p *codexParser) line(b []byte) error {
	var rec codexRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		p.st.NoteUnknown("malformed")
		return nil
	}
	if !rec.Timestamp.IsZero() {
		p.last = rec.Timestamp
	}
	switch rec.Type {
	case "session_meta":
		p.sessionMeta(rec)
	case "turn_context":
		p.turnContext(rec)
	case "event_msg":
		p.eventMsg(rec)
	case "response_item":
		p.responseItem(rec)
	default:
		p.st.NoteUnknown(rec.Type)
	}
	return nil
}

func (p *codexParser) sessionMeta(rec codexRecord) {
	pl := rec.Payload
	p.sess.ID = pl.SessionID
	if p.sess.ID == "" {
		p.sess.ID = pl.ID
	}
	p.sess.CWD = pl.CWD
	if pl.CWD != "" {
		p.sess.Project = filepath.Base(pl.CWD)
	}
	p.sess.GitBranch = pl.Git.Branch
	p.sess.GitSHA = pl.Git.CommitHash
	p.sess.Originator = pl.Originator
	p.sess.CLIVersion = pl.CLIVersion
	p.sess.ParentID = codexParentID(pl.Source)
	p.sess.StartedAt = rec.Timestamp
}

func (p *codexParser) turnContext(rec codexRecord) {
	t, ok := p.byID[rec.Payload.TurnID]
	if !ok {
		t = p.open(rec.Payload.TurnID, true)
	}
	p.cur = t
	if t.model == "" {
		t.model = rec.Payload.Model
	}
	if p.sess.Model == "" {
		p.sess.Model = rec.Payload.Model
	}
}

func (p *codexParser) eventMsg(rec codexRecord) {
	pl := rec.Payload
	switch pl.Type {
	case "user_message":
		p.userMessage(pl.Message)
	case "token_count":
		// Codex re-emits token_count for the same API request: identical last AND
		// an unchanged total. Compare both — two DISTINCT consecutive requests can
		// carry identical last while total advances, and dropping those under-bills.
		if pl.Info.Last != nil {
			cur := codexUsage(*pl.Info.Last)
			var tot model.TokenUsage
			// Older rollouts carry no total; tot stays zero, so two distinct
			// consecutive requests with an identical last are indistinguishable
			// from a re-emit and the second is dropped. Unavoidable without total.
			if pl.Info.Total != nil {
				tot = codexUsage(*pl.Info.Total)
			}
			if p.sawTokens && cur == p.lastTokens && tot == p.lastTotal {
				return
			}
			p.lastTokens, p.lastTotal, p.sawTokens = cur, tot, true
			p.turnFor("").tokens.Add(cur)
		}
	case "task_complete":
		t := p.turnFor(pl.TurnID)
		if pl.DurationMs > 0 {
			t.duration = pl.DurationMs
		}
		t.response = pl.LastAgentMessage
	case "turn_aborted":
		p.turnFor(pl.TurnID).duration = pl.DurationMs
	default:
		p.st.NoteUnknown("event_msg/" + pl.Type)
	}
}

func (p *codexParser) responseItem(rec codexRecord) {
	switch rec.Payload.Type {
	case "custom_tool_call", "function_call":
		p.toolCall(rec)
	case "custom_tool_call_output", "function_call_output":
		p.toolOutput(rec)
	default:
		p.st.NoteUnknown("response_item/" + rec.Payload.Type)
	}
}

func (p *codexParser) userMessage(msg string) {
	if msg == "" {
		return
	}
	// Legacy and subagent rollouts never emit turn_context, so their only turn
	// boundary is the user message itself.
	t := p.cur
	if t == nil || !t.explicit {
		t = p.open("", false)
	}
	t.prompts = append(t.prompts, msg)
}

func (p *codexParser) toolCall(rec codexRecord) {
	pl := rec.Payload
	if pl.CallID == "" {
		return
	}
	raw := pl.Input
	if raw == "" {
		raw = pl.Arguments
	}
	sp := &model.ToolSpan{
		ID:     pl.CallID,
		TurnID: p.turnFor(pl.Meta.TurnID).id,
		Name:   pl.Name,
		Kind:   pl.Type,
		Input:  codexCommand(raw),
		OK:     pl.Status == "" || pl.Status == "completed",
	}
	p.spans = append(p.spans, sp)
	p.calls[pl.CallID] = sp
	p.callAt[pl.CallID] = rec.Timestamp
}

func (p *codexParser) toolOutput(rec codexRecord) {
	sp, ok := p.calls[rec.Payload.CallID]
	if !ok {
		p.st.NoteUnknown("orphan_tool_output")
		return
	}
	sp.Output = codexOutputText(rec.Payload.Output)
	if at, seen := p.callAt[rec.Payload.CallID]; seen && !at.IsZero() && !rec.Timestamp.IsZero() {
		sp.DurationMs = rec.Timestamp.Sub(at).Milliseconds()
	}
}

// turnFor resolves the turn an event belongs to. token_count and user_message
// carry no turn id, and pre-2026-06 rollouts omit it on tool calls too, so those
// fall back to the turn most recently opened. Turns interleave (a turn can
// complete after its successor started), which makes that an approximation.
func (p *codexParser) turnFor(id string) *codexTurn {
	if id != "" {
		if t, ok := p.byID[id]; ok {
			return t
		}
		return p.open(id, true)
	}
	if p.cur != nil {
		return p.cur
	}
	return p.open("", false)
}

func (p *codexParser) open(id string, explicit bool) *codexTurn {
	if id == "" {
		id = "implicit-" + strconv.Itoa(len(p.turns))
	}
	t := &codexTurn{id: id, explicit: explicit, started: p.last}
	p.turns = append(p.turns, t)
	p.byID[id] = t
	p.cur = t
	return t
}

func (p *codexParser) record() model.Record {
	sess := p.sess
	sess.Agent = model.AgentCodex
	if !p.last.IsZero() {
		sess.EndedAt = p.last
	}
	rec := model.Record{Session: sess}
	for i, t := range p.turns {
		rec.Turns = append(rec.Turns, model.Turn{
			ID:           t.id,
			SessionID:    sess.ID,
			Index:        i,
			Model:        t.model,
			PromptText:   strings.Join(t.prompts, "\n"),
			ResponseText: t.response,
			StartedAt:    t.started,
			DurationMs:   t.duration,
			Tokens:       t.tokens,
		})
	}
	for _, sp := range p.spans {
		rec.ToolSpans = append(rec.ToolSpans, *sp)
	}
	return rec
}

// codexUsage maps one last_token_usage. Codex counts cached_input_tokens inside
// input_tokens and reasoning_output_tokens inside output_tokens, but
// TokenUsage.Total adds CacheRead to Input, so the cached share is subtracted
// here to keep the two from being billed twice.
func codexUsage(t codexTokens) model.TokenUsage {
	input := t.Input - t.Cached
	if input < 0 {
		input = 0
	}
	return model.TokenUsage{
		Input:           input,
		Output:          t.Output,
		CacheRead:       t.Cached,
		ReasoningOutput: t.Reasoning,
	}
}

func codexParentID(src json.RawMessage) string {
	if len(src) == 0 {
		return ""
	}
	var v struct {
		Subagent struct {
			ThreadSpawn struct {
				ParentThreadID string `json:"parent_thread_id"`
			} `json:"thread_spawn"`
		} `json:"subagent"`
	}
	if err := json.Unmarshal(src, &v); err != nil {
		return ""
	}
	return v.Subagent.ThreadSpawn.ParentThreadID
}

// codexExecCmdRe pulls the cmd argument out of the JS wrapper Codex sends to the
// exec tool: `tools.exec_command({cmd:"git status",workdir:"/repo"})`.
var codexExecCmdRe = regexp.MustCompile(`exec_command\(\s*\{\s*cmd\s*:\s*("(?:[^"\\]|\\.)*")`)

// codexCommand recovers the shell command from a tool input: JSON arguments on
// the older function_call form, the JS wrapper on custom_tool_call. Inputs that
// are neither (plain scripts against ALL_TOOLS) are kept verbatim.
func codexCommand(raw string) string {
	if raw == "" {
		return ""
	}
	var args struct {
		Cmd string `json:"cmd"`
	}
	if err := json.Unmarshal([]byte(raw), &args); err == nil && args.Cmd != "" {
		return args.Cmd
	}
	m := codexExecCmdRe.FindAllStringSubmatch(raw, -1)
	cmds := make([]string, 0, len(m))
	for _, g := range m {
		var s string
		if err := json.Unmarshal([]byte(g[1]), &s); err != nil {
			s = strings.Trim(g[1], `"`)
		}
		if s != "" {
			cmds = append(cmds, s)
		}
	}
	if len(cmds) == 0 {
		return raw
	}
	return strings.Join(cmds, "\n")
}

// codexOutputText flattens a tool output: a bare string on function_call_output,
// a list of text parts on custom_tool_call_output.
func codexOutputText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return string(raw)
	}
	var b strings.Builder
	for _, part := range parts {
		b.WriteString(part.Text)
	}
	return b.String()
}
