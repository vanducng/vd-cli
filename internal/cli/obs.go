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
	cmd.AddCommand(newObsSessionsCmd(), newObsShowCmd(), newObsUsageCmd())
	return cmd
}

// obsFlags are shared by every subcommand.
type obsFlags struct {
	agent  string
	since  string
	jsonOu bool
	noSync bool
}

func (f *obsFlags) bind(cmd *cobra.Command, sinceDefault string) {
	fl := cmd.Flags()
	fl.StringVar(&f.agent, "agent", "", "Filter by agent: claude-code or codex")
	fl.StringVar(&f.since, "since", sinceDefault, "Only sessions since this time (e.g. 7d, 24h, RFC3339)")
	fl.BoolVar(&f.jsonOu, "json", false, "Emit JSON")
	fl.BoolVar(&f.noSync, "no-sync", false, "Skip the incremental sync and read the cache as-is")
}

// open builds the service and syncs unless told not to. The sync summary goes to
// stderr so it can never corrupt a --json pipe.
func (f *obsFlags) open(ctx context.Context) (*obs.Service, error) {
	if err := checkAgentFlag(f.agent); err != nil {
		return nil, err
	}
	svc, err := obs.NewService("")
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
	if !f.jsonOu && stats.FilesScanned > 0 {
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

func parseSinceFlag(v string) (time.Time, error) {
	if v == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	var n int
	var unit rune
	if _, err := fmt.Sscanf(v, "%d%c", &n, &unit); err != nil || n < 0 {
		return time.Time{}, fmt.Errorf("bad --since %q: want 7d, 24h, or RFC3339", v)
	}
	switch unit {
	case 'd':
		return time.Now().AddDate(0, 0, -n), nil
	case 'h':
		return time.Now().Add(-time.Duration(n) * time.Hour), nil
	}
	return time.Time{}, fmt.Errorf("bad --since %q: want 7d, 24h, or RFC3339", v)
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
			since, err := parseSinceFlag(f.since)
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
			if f.jsonOu {
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
			if f.jsonOu {
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

func newObsUsageCmd() *cobra.Command {
	var f obsFlags
	var daily, monthly bool

	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Report tokens and estimated cost by day or month",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if daily && monthly {
				return fmt.Errorf("--daily and --monthly are mutually exclusive")
			}
			group := store.UsageGroupDaily
			if monthly {
				group = store.UsageGroupMonthly
			}
			since, err := parseSinceFlag(f.since)
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
			if f.jsonOu {
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

func renderSessions(w io.Writer, list *model.SessionList) {
	if len(list.Sessions) == 0 {
		_, _ = fmt.Fprintln(w, "  no sessions found")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "  STARTED\tAGENT\tTITLE\tMODEL\tTURNS\tTOKENS\tEST $")
	for _, s := range list.Sessions {
		_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			s.StartedAt.Local().Format("01-02 15:04"), shortAgent(s.Agent), trunc(s.Title, 24),
			trunc(shortModel(s.Model), 10), s.TurnCount, humanTokens(s.Tokens.Total()), money(s.CostUSD))
	}
	_, _ = fmt.Fprintf(tw, "  \t\t%d of %d\t\t\t\t\n", len(list.Sessions), list.Total)
	_ = tw.Flush()
	_, _ = fmt.Fprintln(w, estNote)
}

func renderSession(w io.Writer, d *model.SessionDetail) {
	_, _ = fmt.Fprintf(w, "  session  %s\tagent   %s\n", d.ID, d.Agent)
	_, _ = fmt.Fprintf(w, "  cwd      %s\tbranch  %s\n", d.CWD, orDash(d.GitBranch))
	_, _ = fmt.Fprintf(w, "  model    %s\tcli     %s\n", orDash(d.Model), orDash(d.CLIVersion))
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
			_, _ = fmt.Fprintf(w, "     skill   %s\n", s.Name)
		}
		for _, sp := range t.ToolSpans {
			if sp.SubagentName == "" || sp.RollupTokens == nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "     agent   %s → %s tok  %s  (subagent rollup)\n",
				sp.SubagentName, humanTokens(sp.RollupTokens.Total()), money(sp.RollupCostUSD))
		}
	}
	if d.TurnCount > len(d.Turns) {
		_, _ = fmt.Fprintf(w, "\n  ... %d more turns    (--turns N to limit · --json for full detail)\n",
			d.TurnCount-len(d.Turns))
	}
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
			len(rep.UnpricedModels), strings.Join(rep.UnpricedModels, ", "))
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

func shortAgent(a string) string { return strings.TrimSuffix(a, "-code") }

func shortModel(m string) string {
	m = strings.TrimPrefix(m, "claude-")
	if i := strings.LastIndex(m, "-2"); i > 0 && len(m)-i >= 7 {
		m = m[:i]
	}
	return m
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
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
		parts = append(parts, fmt.Sprintf("%s %dms", x.HookName, x.DurationMs))
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
		if _, seen := counts[x.Name]; !seen {
			order = append(order, x.Name)
		}
		counts[x.Name]++
		if !x.OK {
			errs = append(errs, x.Name)
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
