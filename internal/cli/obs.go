package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/vanducng/vd-cli/v2/internal/inventory"
	"github.com/vanducng/vd-cli/v2/internal/obs"
	"github.com/vanducng/vd-cli/v2/internal/obs/ingest"
	"github.com/vanducng/vd-cli/v2/internal/obs/model"
	"github.com/vanducng/vd-cli/v2/internal/obs/store"
)

func newObsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "obs",
		Short: "Inspect local Claude Code and Codex sessions, tokens, and cost",
		Long: `Read the transcripts Claude Code and Codex already write under ~/.claude and
~/.codex into a local cache, and report sessions, turns, tool calls, and
API-equivalent cost across both agents.

Read-only: vd never modifies agent-owned files. Costs are estimates computed
from token counts, not a bill.`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error { return c.Help() },
	}
	cmd.AddCommand(newObsSessionsCmd(), newObsShowCmd(), newObsUsageCmd(), newObsHealthCmd())
	return cmd
}

// obsFlags are shared by every subcommand.
type obsFlags struct {
	agent  string
	since  string
	asJSON bool
	noSync bool
}

func (f *obsFlags) bind(cmd *cobra.Command, sinceDefault string) {
	fl := cmd.Flags()
	fl.StringVar(&f.agent, "agent", "", "Filter by agent: claude-code or codex")
	fl.StringVar(&f.since, "since", sinceDefault, "Only sessions since this time (e.g. 7d, 24h, RFC3339)")
	fl.BoolVar(&f.asJSON, "json", false, "Emit JSON")
	fl.BoolVar(&f.noSync, "no-sync", false, "Skip the incremental sync and read the cache as-is")
}

// open builds the service and syncs unless told not to. The sync summary goes to
// stderr so it can never corrupt a --json pipe.
func (f *obsFlags) open(ctx context.Context) (*obs.Service, error) {
	if err := checkAgentFlag(f.agent); err != nil {
		return nil, err
	}
	svc, err := obs.NewService("", healthInventoryService())
	if err != nil {
		return nil, err
	}
	if f.noSync {
		return svc, nil
	}
	stats, err := svc.Sync(ctx, ingest.SyncOptions{})
	if err != nil {
		_ = svc.Close()
		return nil, err
	}
	if !f.asJSON && stats.FilesScanned > 0 {
		_, _ = fmt.Fprintf(os.Stderr, "  sync  %d files parsed · %d cached · %d unknown records       %s\n\n",
			stats.FilesParsed, stats.Skipped, stats.UnknownRecords, stats.Elapsed.Round(time.Millisecond))
	}
	return svc, nil
}

func checkAgentFlag(a string) error {
	switch a {
	case "", model.AgentClaude, model.AgentCodex:
		return nil
	}
	return fmt.Errorf("unknown --agent %q: want claude-code or codex", a)
}

// healthInventoryService resolves the inventory service `vd obs health` uses to
// turn a co-occurring skill name into a file path. Best-effort: unlike `vd
// web`, obs commands work with no repo root at all, so a missing skills.toml
// repo or ~/.claude just means repo-managed skill lookups miss and fall
// through to the platform-home scan — never a hard failure for `vd obs`.
func healthInventoryService() *inventory.Service {
	root, err := resolveRepoRoot(flagRoot)
	if err != nil {
		root = "."
	}
	claudeHome, err := claudeDir()
	if err != nil {
		claudeHome = ""
	}
	return inventory.NewService(root, claudeHome)
}

func newObsSessionsCmd() *cobra.Command {
	var f obsFlags
	var project string
	var query string
	var limit, offset int

	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "List recent sessions across both agents",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			since, err := store.ParseSince(f.since)
			if err != nil {
				return err
			}
			svc, err := f.open(c.Context())
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			list, err := svc.Sessions(c.Context(), model.SessionFilter{
				Agent: f.agent, Project: project, Q: query,
				Since: since, Limit: limit, Offset: offset,
			})
			if err != nil {
				return err
			}
			if f.asJSON {
				return emitJSON(c.OutOrStdout(), list)
			}
			renderSessions(c.OutOrStdout(), list)
			return nil
		},
	}
	f.bind(cmd, "7d")
	cmd.Flags().StringVar(&project, "project", "", "Filter by project name or cwd prefix")
	cmd.Flags().StringVar(&query, "query", "", "Filter by title or cwd substring")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max rows to show")
	cmd.Flags().IntVar(&offset, "offset", 0, "Skip this many rows")
	return cmd
}

func newObsShowCmd() *cobra.Command {
	var f obsFlags
	var turns, offset int

	cmd := &cobra.Command{
		Use:   "show <session-id-or-prefix>",
		Short: "Show one session turn by turn",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			svc, err := f.open(c.Context())
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			d, err := svc.Session(c.Context(), args[0], f.agent, turns, offset)
			switch {
			case err == nil:
			case errors.Is(err, obs.ErrNotFound):
				return fmt.Errorf("no session matches %q", args[0])
			case errors.Is(err, obs.ErrTooShort):
				return fmt.Errorf("id prefix %q is too short: give at least %d characters (codex ids all begin 019...)", args[0], store.MinPrefixLen)
			case errors.Is(err, obs.ErrAmbiguous):
				return fmt.Errorf("id prefix %q matches more than one session: give more characters, or narrow with --agent", args[0])
			default:
				return err
			}
			if f.asJSON {
				return emitJSON(c.OutOrStdout(), d)
			}
			renderSession(c.OutOrStdout(), d)
			return nil
		},
	}
	f.bind(cmd, "")
	cmd.Flags().IntVar(&turns, "turns", 0, "Show at most this many turns")
	cmd.Flags().IntVar(&offset, "offset", 0, "Skip this many turns")
	return cmd
}

func newObsHealthCmd() *cobra.Command {
	var f obsFlags
	var project string

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Investigate signal: recurring tool errors, not a health verdict",
		Long: `Aggregate tool-call failures (tool_spans where ok=false) into recurring
error clusters. Agents fail-probe routinely — a grep no-match or an
exploratory Bash call both trip ok=false — so a high count means "look here",
never "this is broken". Each cluster carries fetchable evidence
(vd obs show <session-id> --json) and, where the error text names a skill, a
resolved file path to start from.

Self-heal loop: pick the top cluster, fetch its evidence turns, edit the
linked file, then re-run with a tight --since window — old errors persist
inside wide windows, so the signature is the tracking key, not the count.`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			since, err := store.ParseSince(f.since)
			if err != nil {
				return err
			}
			svc, err := f.open(c.Context())
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			rep, err := svc.Health(c.Context(), model.HealthFilter{Since: since, Agent: f.agent, Project: project})
			if err != nil {
				return err
			}
			if f.asJSON {
				return emitJSON(c.OutOrStdout(), rep)
			}
			renderHealth(c.OutOrStdout(), rep)
			return nil
		},
	}
	f.bind(cmd, "7d")
	cmd.Flags().StringVar(&project, "project", "", "Filter by project name or cwd prefix")
	return cmd
}

func newObsUsageCmd() *cobra.Command {
	var f obsFlags
	var daily, monthly bool

	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Report tokens and estimated cost by day or month",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			// --daily defaults to true, so test what the user actually set, not the
			// value: otherwise `--monthly` alone reads as "both" and errors.
			if c.Flags().Changed("daily") && monthly {
				return fmt.Errorf("--daily and --monthly are mutually exclusive")
			}
			group := store.UsageGroupDaily
			if monthly {
				group = store.UsageGroupMonthly
			}
			since, err := store.ParseSince(f.since)
			if err != nil {
				return err
			}
			svc, err := f.open(c.Context())
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			rep, err := svc.Usage(c.Context(), model.UsageFilter{Group: group, Agent: f.agent, Since: since})
			if err != nil {
				return err
			}
			if f.asJSON {
				return emitJSON(c.OutOrStdout(), rep)
			}
			renderUsage(c.OutOrStdout(), rep)
			return nil
		},
	}
	f.bind(cmd, "7d")
	cmd.Flags().BoolVar(&daily, "daily", true, "Group by day")
	cmd.Flags().BoolVar(&monthly, "monthly", false, "Group by month")
	return cmd
}

func emitJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

const estNote = "  est $ = API-equivalent from token counts, not a subscription bill."

// sanitize strips control bytes from transcript-derived strings before they hit
// the terminal: titles, tool names, models etc. come from files other agents
// wrote, and raw C0/C1 output lets an injected transcript retitle the terminal,
// clear the screen, or write the clipboard via OSC52.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return -1
		}
		return r
	}, s)
}

func renderSessions(w io.Writer, list *model.SessionList) {
	if len(list.Sessions) == 0 {
		_, _ = fmt.Fprintln(w, "  no sessions found")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "  STARTED\tAGENT\tTITLE\tMODEL\tTURNS\tTOKENS\tEST $")
	for _, s := range list.Sessions {
		_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			s.StartedAt.Local().Format("01-02 15:04"), shortAgent(s.Agent), trunc(sanitize(s.Title), 24),
			trunc(shortModel(s.Model), 10), s.TurnCount, humanTokens(s.Tokens.Total()), money(s.CostUSD))
	}
	_, _ = fmt.Fprintf(tw, "  \t\t%d of %d\t\t\t\t\n", len(list.Sessions), list.Total)
	_ = tw.Flush()
	_, _ = fmt.Fprintln(w, estNote)
}

func renderSession(w io.Writer, d *model.SessionDetail) {
	_, _ = fmt.Fprintf(w, "  session  %s\tagent   %s\n", sanitize(d.ID), sanitize(d.Agent))
	_, _ = fmt.Fprintf(w, "  cwd      %s\tbranch  %s\n", sanitize(d.CWD), orDash(sanitize(d.GitBranch)))
	_, _ = fmt.Fprintf(w, "  model    %s\tcli     %s\n", orDash(sanitize(d.Model)), orDash(sanitize(d.CLIVersion)))
	cache := "-"
	if d.CacheHitRate != nil {
		cache = fmt.Sprintf("%.0f%% cache hit", *d.CacheHitRate*100)
	}
	_, _ = fmt.Fprintf(w, "  totals   %d turns · %s tok · %s est · %s\n",
		d.TurnCount, humanTokens(d.Tokens.Total()), money(d.CostUSD), cache)

	for _, t := range d.Turns {
		_, _ = fmt.Fprintf(w, "\n  ── turn %d ── %s  %s  %s tok  %s\n", t.Index+1,
			t.StartedAt.Local().Format("15:04:05"), dur(t.DurationMs),
			humanTokens(t.Tokens.Total()), money(t.CostUSD))
		if p := firstLine(t.PromptText); p != "" {
			_, _ = fmt.Fprintf(w, "     prompt  %q\n", trunc(p, 60))
		}
		if len(t.HookExecs) > 0 {
			_, _ = fmt.Fprintf(w, "     hooks   %s\n", joinHooks(t.HookExecs))
		}
		if len(t.ToolSpans) > 0 {
			_, _ = fmt.Fprintf(w, "     tools   %s\n", joinSpans(t.ToolSpans))
		}
		for _, s := range t.Skills {
			_, _ = fmt.Fprintf(w, "     skill   %s\n", sanitize(s.Name))
		}
		for _, sp := range t.ToolSpans {
			if sp.SubagentName == "" || sp.RollupTokens == nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "     agent   %s → %s tok  %s  (subagent rollup)\n",
				sanitize(sp.SubagentName), humanTokens(sp.RollupTokens.Total()), money(sp.RollupCostUSD))
		}
	}
	if d.TurnCount > len(d.Turns) {
		_, _ = fmt.Fprintf(w, "\n  ... %d more turns    (--turns N to limit · --json for full detail)\n",
			d.TurnCount-len(d.Turns))
	}
}

// renderHealth frames the report as an investigate signal, never a verdict:
// the header line is load-bearing copy, not decoration.
func renderHealth(w io.Writer, rep *model.HealthReport) {
	_, _ = fmt.Fprintln(w, "  INVESTIGATE SIGNAL — not a health verdict: agents fail-probe routinely.")
	_, _ = fmt.Fprintf(w, "  window   %s -> %s\n",
		rep.Window.Since.Local().Format("01-02 15:04"), rep.Window.Until.Local().Format("01-02 15:04"))
	delta := "n/a"
	if rep.Delta != nil {
		delta = fmt.Sprintf("%+d", *rep.Delta)
	}
	_, _ = fmt.Fprintf(w, "  errors   %d (%.1f%% of tool calls)   delta %s   sessions with errors %d\n\n",
		rep.TotalErrors, rep.ErrorRate*100, delta, rep.ErroredSessions)

	if len(rep.Clusters) == 0 {
		_, _ = fmt.Fprintln(w, "  no recurring error clusters in this window")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "  COUNT\tTREND\tTOOLS\tSKILL PATH\tSIGNATURE")
	for _, c := range rep.Clusters {
		trend := c.Trend
		if trend == "" {
			trend = "-"
		}
		skill := "-"
		if c.SuggestedFocus != nil {
			skill = sanitize(*c.SuggestedFocus)
		}
		_, _ = fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\t%s\n",
			c.Count, trend, trunc(sanitize(strings.Join(c.AffectedTools, ",")), 20),
			trunc(skill, 30), trunc(sanitize(c.Signature), 60))
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintln(w, "\n  investigate: vd obs show <session-id> --json   ·   verify with a tight --since after a fix")
}

func renderUsage(w io.Writer, rep *model.UsageReport) {
	if len(rep.Rows) == 0 {
		_, _ = fmt.Fprintln(w, "  no usage in range")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "  DATE\tAGENT\tMODEL\tINPUT\tOUTPUT\tCACHE R\tCACHE W\tEST $")
	for _, r := range rep.Rows {
		_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.Date, shortAgent(r.Agent),
			trunc(shortModel(r.Model), 12), humanTokens(r.Tokens.Input), humanTokens(r.Tokens.Output),
			humanTokens(r.Tokens.CacheRead), humanTokens(r.Tokens.CacheWrite), money(r.CostUSD))
	}
	_, _ = fmt.Fprintf(tw, "  \t\tTOTAL\t%s\t%s\t%s\t%s\t%s\n", humanTokens(rep.Totals.Input),
		humanTokens(rep.Totals.Output), humanTokens(rep.Totals.CacheRead),
		humanTokens(rep.Totals.CacheWrite), money(rep.TotalCostUSD))
	_ = tw.Flush()

	if len(rep.UnpricedModels) > 0 {
		_, _ = fmt.Fprintf(w, "\n  ! %d unpriced model(s): %s   (add to ~/.vd/obs/prices.json)\n",
			len(rep.UnpricedModels), sanitize(strings.Join(rep.UnpricedModels, ", ")))
	}
	_, _ = fmt.Fprintln(w, estNote)
}

// money renders an unpriced model as "?" — never 0, which reads as free.
func money(v *float64) string {
	if v == nil {
		return "?"
	}
	return fmt.Sprintf("%.2f", *v)
}

func humanTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func dur(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	if d >= time.Minute {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func shortAgent(a string) string { return strings.TrimSuffix(sanitize(a), "-code") }

func shortModel(m string) string {
	m = strings.TrimPrefix(sanitize(m), "claude-")
	if i := strings.LastIndex(m, "-2"); i > 0 && len(m)-i >= 7 {
		m = m[:i]
	}
	return m
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func joinHooks(h []model.HookExec) string {
	parts := make([]string, 0, len(h))
	for _, x := range h {
		parts = append(parts, fmt.Sprintf("%s %dms", sanitize(x.HookName), x.DurationMs))
	}
	return strings.Join(parts, " · ")
}

// joinSpans summarizes rather than lists: a real turn runs dozens of tools and a
// full enumeration wraps off the terminal. Errors are always named — that is what
// you are scanning for.
func joinSpans(s []model.ToolSpan) string {
	counts := map[string]int{}
	order := []string{}
	errs := []string{}
	for _, x := range s {
		name := sanitize(x.Name)
		if _, seen := counts[name]; !seen {
			order = append(order, name)
		}
		counts[name]++
		if !x.OK {
			errs = append(errs, name)
		}
	}
	parts := make([]string, 0, len(order))
	for _, n := range order {
		if counts[n] > 1 {
			parts = append(parts, fmt.Sprintf("%s×%d", n, counts[n]))
			continue
		}
		parts = append(parts, n)
	}
	out := strings.Join(parts, " · ")
	if len(errs) > 0 {
		out += fmt.Sprintf("   (%d err: %s)", len(errs), strings.Join(dedupe(errs), ", "))
	}
	return out
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
