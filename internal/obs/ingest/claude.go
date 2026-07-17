package ingest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
)

const originHuman = "human"

// ClaudeRoot resolves the Claude home, mirroring internal/cli.claudeDir(). The repo
// already carries three agent-home resolution schemes; a fourth env var is DRY debt.
func ClaudeRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

// EnumerateClaude splits the transcript tree into the sessions worth listing and the
// subagent files that only contribute usage.
//
// Measured locally: 236 top-level transcripts against 3798 subagent files, so
// globbing **/*.jsonl reports ~17x the sessions that exist — message.id dedupe
// cannot repair that, because a subagent's API calls carry genuinely distinct ids.
// Subagents nest at two depths (subagents/ and subagents/workflows/wf_*/), hence the
// walk rather than a fixed glob. The agent- prefix is load-bearing: journal.jsonl
// shares those directories but carries no agentId, so parsing one as a session would
// give it its parent's sessionId and overwrite the parent's row on upsert.
func EnumerateClaude(root string) (top []string, subagents []string, err error) {
	projects := filepath.Join(root, "projects")
	top, err = filepath.Glob(filepath.Join(projects, "*", "*.jsonl"))
	if err != nil {
		return nil, nil, fmt.Errorf("glob claude sessions: %w", err)
	}
	if err := filepath.WalkDir(projects, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, "agent-") && strings.HasSuffix(name, ".jsonl") &&
			hasSegment(path, "subagents") {
			subagents = append(subagents, path)
		}
		return nil
	}); err != nil {
		return nil, nil, fmt.Errorf("walk claude subagents: %w", err)
	}
	return top, subagents, nil
}

// ParseClaudeFile parses the whole transcript at path. WorkflowID exists only in
// the path — no record carries it.
func ParseClaudeFile(path string, st *ScanState) (model.Record, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return model.Record{}, 0, fmt.Errorf("open transcript %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	rec, off, err := ParseClaude(f, st)
	sess := &rec.Session
	if sess.WorkflowID == "" {
		sess.WorkflowID = workflowIDFromPath(path)
	}
	// A subagent file whose records carry no agentId would keep its parent's
	// sessionId as its own id and overwrite the parent's row; the filename holds the
	// same id, so prefer being wrong about the name over corrupting the parent.
	if sess.ParentID == "" && hasSegment(path, "subagents") {
		id := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), "agent-"), ".jsonl")
		if id != "" && id != sess.ID {
			sess.ParentID, sess.ID = sess.ID, id
			for i := range rec.Turns {
				rec.Turns[i].SessionID = id
			}
		}
	}
	namespaceTurnIDs(&rec)
	return rec, off, err
}

// namespaceTurnIDs prefixes every turn id with the session's FINAL identity.
// A parent and all subagents it spawns share one promptId, so an un-namespaced
// turns.id collides across those files and the store's upsert lets the last
// writer erase the others' tokens — measured as a 50-70% assistant-heavy
// undercount on fan-out-heavy corpora. Must run after the subagent identity
// swap above, or every sibling would share the parent's prefix and collide anyway.
func namespaceTurnIDs(rec *model.Record) {
	if rec.Session.ID == "" {
		return
	}
	// Always prefix — a skip-if-prefixed check makes the mapping non-injective
	// (raw key "sess:p1" would collide with key "p1" of session "sess").
	prefix := rec.Session.ID + ":"
	remap := make(map[string]string, len(rec.Turns))
	for i := range rec.Turns {
		old := rec.Turns[i].ID
		id := prefix + old
		remap[old] = id
		rec.Turns[i].ID = id
	}
	if len(remap) == 0 {
		return
	}
	for i := range rec.ToolSpans {
		if id, ok := remap[rec.ToolSpans[i].TurnID]; ok {
			rec.ToolSpans[i].TurnID = id
		}
	}
	for i := range rec.HookExecs {
		if id, ok := remap[rec.HookExecs[i].TurnID]; ok {
			rec.HookExecs[i].TurnID = id
		}
	}
	for i := range rec.Skills {
		if id, ok := remap[rec.Skills[i].TurnID]; ok {
			rec.Skills[i].TurnID = id
		}
	}
}

// ParseClaude reads a whole transcript into a record, committing only complete
// lines. Malformed lines and unmodelled record types are skipped and counted on st,
// so schema drift surfaces as a number rather than as missing data.
//
// r must start at the beginning of the file. Resuming mid-file would re-emit a turn
// holding only post-offset tokens, and the store's upsert replaces rather than
// merges — stored totals would shrink. Sync skips unchanged files instead.
func ParseClaude(r io.Reader, st *ScanState) (model.Record, int64, error) {
	p := &claudeParser{st: st, turnIdx: -1, spans: map[string]int{}}
	p.rec.Session.Agent = model.AgentClaude
	off, oversized, err := ScanLines(r, 0, p.line)
	for i := 0; i < oversized; i++ {
		p.st.NoteUnknown("oversized_line")
	}
	p.closeTurn()
	p.finish()
	st.Offset = off
	return p.rec, off, err
}

type claudeParser struct {
	st        *ScanState
	rec       model.Record
	sessionID string
	agentID   string
	turnIdx   int
	turnKey   string
	spans     map[string]int
	billed    map[string]model.TokenUsage
	hookSeq   map[string]int
	skillSeq  map[string]int
	lastTS    time.Time
}

// billDelta adds only the growth of a message's usage to the turn. Fields are
// clamped non-negative so an out-of-order smaller snapshot can never subtract.
func (p *claudeParser) billDelta(t *model.Turn, msgID string, u model.TokenUsage) {
	if p.billed == nil {
		p.billed = map[string]model.TokenUsage{}
	}
	prev := p.billed[msgID]
	d := model.TokenUsage{
		Input:           maxInt(u.Input-prev.Input, 0),
		Output:          maxInt(u.Output-prev.Output, 0),
		CacheRead:       maxInt(u.CacheRead-prev.CacheRead, 0),
		CacheWrite:      maxInt(u.CacheWrite-prev.CacheWrite, 0),
		ReasoningOutput: maxInt(u.ReasoningOutput-prev.ReasoningOutput, 0),
	}
	t.Tokens.Add(d)
	prev.Input = maxInt(prev.Input, u.Input)
	prev.Output = maxInt(prev.Output, u.Output)
	prev.CacheRead = maxInt(prev.CacheRead, u.CacheRead)
	prev.CacheWrite = maxInt(prev.CacheWrite, u.CacheWrite)
	prev.ReasoningOutput = maxInt(prev.ReasoningOutput, u.ReasoningOutput)
	p.billed[msgID] = prev
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// cur indexes rather than holds a *model.Turn: appending to rec.Turns reallocates,
// which would silently strand writes through a cached pointer.
func (p *claudeParser) cur() *model.Turn {
	if p.turnIdx < 0 {
		return nil
	}
	return &p.rec.Turns[p.turnIdx]
}

func (p *claudeParser) line(b []byte) error {
	if len(bytes.TrimSpace(b)) == 0 {
		return nil
	}
	var l claudeLine
	if err := json.Unmarshal(b, &l); err != nil {
		p.st.NoteUnknown("malformed")
		return nil
	}
	ts := p.header(&l)
	// Only conversation records extend a turn: trailing metadata (ai-title, hooks)
	// clusters after the last reply and would pad the duration fallback.
	if !ts.IsZero() && (l.Type == "user" || l.Type == "assistant") {
		p.lastTS = ts
	}
	switch l.Type {
	case "user":
		p.user(&l)
	case "assistant":
		p.assistant(&l)
	case "system":
		p.system(&l)
	case "attachment":
		p.attachment(&l)
	case "ai-title":
		if l.AiTitle != "" {
			p.rec.Session.Title = l.AiTitle
		}
	default:
		p.st.NoteUnknown(l.Type)
	}
	return nil
}

func (p *claudeParser) header(l *claudeLine) time.Time {
	setIfEmpty(&p.sessionID, l.SessionID)
	setIfEmpty(&p.agentID, l.AgentID)
	s := &p.rec.Session
	setIfEmpty(&s.CWD, l.CWD)
	setIfEmpty(&s.GitBranch, l.GitBranch)
	setIfEmpty(&s.CLIVersion, l.Version)
	setIfEmpty(&s.Originator, l.Entrypoint)
	ts := parseTS(l.Timestamp)
	if ts.IsZero() {
		return ts
	}
	if s.StartedAt.IsZero() || ts.Before(s.StartedAt) {
		s.StartedAt = ts
	}
	if ts.After(s.EndedAt) {
		s.EndedAt = ts
	}
	return ts
}

func (p *claudeParser) user(l *claudeLine) {
	if l.PromptID != "" && l.PromptID != p.turnKey {
		p.openTurn(l.PromptID, l)
	}
	if l.Message == nil {
		return
	}
	blocks := decodeBlocks(l.Message.Content)
	if l.Origin != nil && l.Origin.Kind == originHuman {
		if t := p.cur(); t != nil {
			appendText(&t.PromptText, contentText(l.Message.Content, blocks))
		}
	}
	for i := range blocks {
		if blocks[i].Type == "tool_result" {
			p.toolResult(l, &blocks[i])
		}
	}
}

func (p *claudeParser) assistant(l *claudeLine) {
	if p.cur() == nil {
		// Subagent transcripts hold no origin.kind=="human" record, so without an
		// implicit turn their usage would have nowhere to land and drop out of billing.
		p.openTurn("", l)
	}
	t := p.cur()
	if l.Message == nil {
		return
	}
	if l.Message.Model != "" {
		t.Model = l.Message.Model
		p.rec.Session.Model = l.Message.Model
	}
	// One JSONL record per content block repeats the usage object — but not
	// verbatim: output_tokens GROWS as blocks stream, and only the last record
	// carries the reply's true total (measured: 24% of message ids differ,
	// first-wins under-bills output ~46%). Bill the monotonic delta per field so
	// a message is counted once at its final size regardless of record order.
	if l.Message.Usage != nil && l.Message.ID != "" {
		p.billDelta(t, l.Message.ID, l.Message.Usage.tokens())
	}
	blocks := decodeBlocks(l.Message.Content)
	for i := range blocks {
		switch blocks[i].Type {
		case "text":
			appendText(&t.ResponseText, blocks[i].Text)
		case "tool_use":
			p.toolUse(t.ID, &blocks[i])
		}
	}
}

func (p *claudeParser) system(l *claudeLine) {
	if l.Subtype != "turn_duration" {
		p.st.NoteUnknown("system/" + l.Subtype)
		return
	}
	if t := p.cur(); t != nil && l.DurationMs > 0 {
		t.DurationMs = l.DurationMs
	}
}

func (p *claudeParser) attachment(l *claudeLine) {
	if l.Attachment == nil {
		p.st.NoteUnknown("attachment")
		return
	}
	if l.Attachment.Type != "hook_success" {
		p.st.NoteUnknown("attachment/" + l.Attachment.Type)
		return
	}
	// hook_execs is keyed (turn_id, hook_name, event) with no session_id, so a
	// session-scoped hook firing before the first turn would collide across sessions.
	t := p.cur()
	if t == nil {
		return
	}
	if p.hookSeq == nil {
		p.hookSeq = map[string]int{}
	}
	key := t.ID + "|" + l.Attachment.HookName + "|" + l.Attachment.HookEvent
	seq := p.hookSeq[key]
	p.hookSeq[key] = seq + 1
	p.rec.HookExecs = append(p.rec.HookExecs, model.HookExec{
		TurnID:     t.ID,
		HookName:   l.Attachment.HookName,
		Event:      l.Attachment.HookEvent,
		Seq:        seq,
		DurationMs: l.Attachment.DurationMs,
		ExitCode:   l.Attachment.ExitCode,
	})
}

func (p *claudeParser) toolUse(turnID string, b *claudeBlock) {
	if b.ID == "" {
		return
	}
	span := model.ToolSpan{
		ID:     b.ID,
		TurnID: turnID,
		Name:   b.Name,
		Kind:   toolKind(b.Name),
		Input:  string(b.Input),
	}
	switch b.Name {
	case "Skill":
		var in claudeSkillInput
		if err := json.Unmarshal(b.Input, &in); err == nil && in.Skill != "" {
			if p.skillSeq == nil {
				p.skillSeq = map[string]int{}
			}
			sk := turnID + "|" + in.Skill
			seq := p.skillSeq[sk]
			p.skillSeq[sk] = seq + 1
			p.rec.Skills = append(p.rec.Skills, model.Skill{
				TurnID: turnID, Name: in.Skill, Seq: seq, Args: in.Args,
			})
		}
	case "Task", "Agent":
		var in claudeTaskInput
		if err := json.Unmarshal(b.Input, &in); err == nil {
			span.SubagentName = in.SubagentType
		}
	}
	p.spans[b.ID] = len(p.rec.ToolSpans)
	p.rec.ToolSpans = append(p.rec.ToolSpans, span)
}

func (p *claudeParser) toolResult(l *claudeLine, b *claudeBlock) {
	idx, ok := p.spans[b.ToolUseID]
	if !ok {
		return
	}
	span := &p.rec.ToolSpans[idx]
	text := contentText(b.Content, decodeBlocks(b.Content))
	if b.IsError {
		span.Error = text
	} else {
		span.Output = text
		span.OK = true
	}
	p.rollup(span, l)
}

// rollup records what a parent saw of its subagent. The tokens are display-only:
// they are billed from the subagent's own transcript, and adding both is the
// double-count trap, so RollupTokens never enters a sum.
func (p *claudeParser) rollup(span *model.ToolSpan, l *claudeLine) {
	raw := bytes.TrimSpace(l.ToolUseResult)
	if len(raw) == 0 || raw[0] != '{' {
		return
	}
	var r claudeRollup
	if err := json.Unmarshal(raw, &r); err != nil || r.AgentID == "" {
		return
	}
	span.SubagentSessionID = r.AgentID
	if r.AgentType != "" {
		span.SubagentName = r.AgentType
	}
	if r.TotalDurationMs > 0 {
		span.DurationMs = r.TotalDurationMs
	}
	if r.Usage != nil {
		t := r.Usage.tokens()
		span.RollupTokens = &t
	}
}

func (p *claudeParser) openTurn(key string, l *claudeLine) {
	p.closeTurn()
	id := key
	if id == "" {
		id = l.UUID
	}
	if id == "" {
		id = fmt.Sprintf("%s:%d", p.sessionID, len(p.rec.Turns))
	}
	p.rec.Turns = append(p.rec.Turns, model.Turn{
		ID:        id,
		Index:     len(p.rec.Turns),
		StartedAt: parseTS(l.Timestamp),
	})
	p.turnIdx = len(p.rec.Turns) - 1
	p.turnKey = key
}

func (p *claudeParser) closeTurn() {
	t := p.cur()
	if t == nil {
		return
	}
	if t.DurationMs == 0 && !t.StartedAt.IsZero() && p.lastTS.After(t.StartedAt) {
		t.DurationMs = p.lastTS.Sub(t.StartedAt).Milliseconds()
	}
	p.turnIdx = -1
	p.turnKey = ""
}

func (p *claudeParser) finish() {
	s := &p.rec.Session
	s.ID = p.sessionID
	if p.agentID != "" {
		// A subagent transcript's sessionId field names its parent, not itself; its
		// own identity is agentId, which is also the filename.
		s.ID = p.agentID
		s.ParentID = p.sessionID
	}
	if s.CWD != "" {
		// The project directory is a lossy mangling of cwd (- for every /), and the
		// store filters `project = ? OR cwd LIKE ?`, so project must be the short name.
		s.Project = filepath.Base(s.CWD)
	}
	for i := range p.rec.Turns {
		p.rec.Turns[i].SessionID = s.ID
	}
}

func toolKind(name string) string {
	switch {
	case strings.HasPrefix(name, "mcp__"):
		return "mcp"
	case name == "Task" || name == "Agent":
		return "subagent"
	default:
		return "builtin"
	}
}

func hasSegment(path, seg string) bool {
	for _, p := range strings.Split(filepath.ToSlash(path), "/") {
		if p == seg {
			return true
		}
	}
	return false
}

func workflowIDFromPath(path string) string {
	for _, p := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.HasPrefix(p, "wf_") {
			return p
		}
	}
	return ""
}

// decodeBlocks returns the content blocks, or nil when content is a bare string —
// message.content and tool_result.content are each both shapes in the real corpus.
func decodeBlocks(raw json.RawMessage) []claudeBlock {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil
	}
	var blocks []claudeBlock
	if err := json.Unmarshal(trimmed, &blocks); err != nil {
		return nil
	}
	return blocks
}

func contentText(raw json.RawMessage, blocks []claudeBlock) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return ""
		}
		return s
	}
	var out string
	for i := range blocks {
		if blocks[i].Type == "text" {
			appendText(&out, blocks[i].Text)
		}
	}
	return out
}

func appendText(dst *string, s string) {
	switch {
	case s == "":
	case *dst == "":
		*dst = s
	default:
		*dst += "\n" + s
	}
}

func setIfEmpty(dst *string, v string) {
	if *dst == "" && v != "" {
		*dst = v
	}
}

func parseTS(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

type claudeLine struct {
	Type          string            `json:"type"`
	Subtype       string            `json:"subtype"`
	SessionID     string            `json:"sessionId"`
	AgentID       string            `json:"agentId"`
	UUID          string            `json:"uuid"`
	PromptID      string            `json:"promptId"`
	Timestamp     string            `json:"timestamp"`
	CWD           string            `json:"cwd"`
	GitBranch     string            `json:"gitBranch"`
	Version       string            `json:"version"`
	Entrypoint    string            `json:"entrypoint"`
	AiTitle       string            `json:"aiTitle"`
	DurationMs    int64             `json:"durationMs"`
	Origin        *claudeOrigin     `json:"origin"`
	Message       *claudeMessage    `json:"message"`
	Attachment    *claudeAttachment `json:"attachment"`
	ToolUseResult json.RawMessage   `json:"toolUseResult"`
}

type claudeOrigin struct {
	Kind string `json:"kind"`
}

type claudeMessage struct {
	ID      string          `json:"id"`
	Model   string          `json:"model"`
	Usage   *claudeUsage    `json:"usage"`
	Content json.RawMessage `json:"content"`
}

// claudeUsage omits reasoning: Claude bills thinking inside output_tokens, and
// model.TokenUsage.Total already counts Output.
type claudeUsage struct {
	Input      int `json:"input_tokens"`
	Output     int `json:"output_tokens"`
	CacheRead  int `json:"cache_read_input_tokens"`
	CacheWrite int `json:"cache_creation_input_tokens"`
}

func (u claudeUsage) tokens() model.TokenUsage {
	return model.TokenUsage{
		Input:      u.Input,
		Output:     u.Output,
		CacheRead:  u.CacheRead,
		CacheWrite: u.CacheWrite,
	}
}

type claudeBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

type claudeAttachment struct {
	Type       string `json:"type"`
	HookName   string `json:"hookName"`
	HookEvent  string `json:"hookEvent"`
	DurationMs int64  `json:"durationMs"`
	ExitCode   int    `json:"exitCode"`
}

type claudeRollup struct {
	AgentID         string       `json:"agentId"`
	AgentType       string       `json:"agentType"`
	Usage           *claudeUsage `json:"usage"`
	TotalDurationMs int64        `json:"totalDurationMs"`
}

type claudeSkillInput struct {
	Skill string `json:"skill"`
	Args  string `json:"args"`
}

type claudeTaskInput struct {
	SubagentType string `json:"subagent_type"`
}
