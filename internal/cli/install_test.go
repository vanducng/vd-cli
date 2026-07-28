package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunInstallCodex_DryRunOutput(t *testing.T) {
	root := setupE2ERepo(t)
	dest := filepath.Join(t.TempDir(), "codex-skills")
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := runInstallCodex(cmd, root, []string{"foo"}, installOptions{
		dest:   dest,
		dryRun: true,
	})
	if err != nil {
		t.Fatalf("runInstallCodex: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "would symlink codex skill foo -> "+filepath.Join(dest, "foo")) {
		t.Fatalf("output = %q", got)
	}
}

func TestRunInstallDroid_DryRunOutput(t *testing.T) {
	root := setupE2ERepo(t)
	dest := filepath.Join(t.TempDir(), "droid-skills")
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := runInstallDroid(cmd, root, []string{"foo"}, installOptions{
		dest:   dest,
		dryRun: true,
	})
	if err != nil {
		t.Fatalf("runInstallDroid: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "would symlink droid skill foo -> "+filepath.Join(dest, "foo")) {
		t.Fatalf("output = %q", got)
	}
}

func TestRunInstallClaude_DryRunOutput(t *testing.T) {
	root := setupE2ERepo(t)
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := runInstallClaude(cmd, root, installOptions{scope: "user", dryRun: true}); err != nil {
		t.Fatalf("runInstallClaude: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"would run: vd build claude",
		"would run: claude plugin marketplace add --scope user " + root,
		"would run: claude plugin install --scope user test-bundle@test-skills",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestResolveInstallSelection(t *testing.T) {
	tests := []struct {
		name      string
		selection string
		wantAgent string
		wantScope string
		wantCopy  bool
	}{
		{
			name:      "codex user symlink is default choice",
			selection: "1",
			wantAgent: "codex",
			wantScope: "user",
		},
		{
			name:      "codex repo symlink",
			selection: "codex repo",
			wantAgent: "codex",
			wantScope: "repo",
		},
		{
			name:      "codex snapshot copy",
			selection: "snapshot",
			wantAgent: "codex",
			wantScope: "user",
			wantCopy:  true,
		},
		{
			name:      "claude plugin",
			selection: "claude-code",
			wantAgent: "claude",
			wantScope: "project",
		},
		{
			name:      "droid repo symlink",
			selection: "7",
			wantAgent: "droid",
			wantScope: "repo",
		},
		{
			name:      "droid snapshot copy",
			selection: "droid snapshot",
			wantAgent: "droid",
			wantScope: "user",
			wantCopy:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, opts, err := resolveInstallSelection(tt.selection, installOptions{scope: "project"})
			if err != nil {
				t.Fatalf("resolveInstallSelection: %v", err)
			}
			if agent != tt.wantAgent {
				t.Fatalf("agent = %q, want %q", agent, tt.wantAgent)
			}
			if opts.scope != tt.wantScope {
				t.Fatalf("scope = %q, want %q", opts.scope, tt.wantScope)
			}
			if opts.copy != tt.wantCopy {
				t.Fatalf("copy = %v, want %v", opts.copy, tt.wantCopy)
			}
		})
	}
}

func TestResolveInstallSelection_ClaudeClearsCodexFlags(t *testing.T) {
	in := installOptions{
		scope: "user",
		dest:  "/tmp/somewhere",
		copy:  true,
		force: true,
	}
	agent, opts, err := resolveInstallSelection("4", in)
	if err != nil {
		t.Fatalf("resolveInstallSelection: %v", err)
	}
	if agent != "claude" {
		t.Fatalf("agent = %q, want claude", agent)
	}
	if opts.copy || opts.force || opts.dest != "" {
		t.Fatalf("codex-only flags not cleared: %+v", opts)
	}
}

func TestResolveInstallSelection_ClaudeRewritesRepoScope(t *testing.T) {
	// `--scope repo` is codex-only; if the user mixed it with a claude
	// pick, fall back to the safe default rather than carrying it over.
	agent, opts, err := resolveInstallSelection("claude", installOptions{scope: "repo"})
	if err != nil {
		t.Fatalf("resolveInstallSelection: %v", err)
	}
	if agent != "claude" || opts.scope != "user" {
		t.Fatalf("agent=%q scope=%q, want claude/user", agent, opts.scope)
	}
}

func TestRunInstallClaudeDev_DryRunOutput(t *testing.T) {
	root := setupE2ERepo(t)
	dest := filepath.Join(t.TempDir(), "claude-skills")
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := runInstallClaudeDev(cmd, root, []string{"foo"}, installOptions{
		dest:   dest,
		dryRun: true,
		dev:    true,
	})
	if err != nil {
		t.Fatalf("runInstallClaudeDev: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "would symlink claude skill foo -> "+filepath.Join(dest, "foo")) {
		t.Fatalf("output = %q", got)
	}
}

func TestRunInstall_ClaudeDevAcceptsSkillNames(t *testing.T) {
	root := setupE2ERepo(t)
	dest := filepath.Join(t.TempDir(), "claude-skills")
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := runInstall(cmd, root, []string{"claude", "foo"}, installOptions{
		scope:  "user",
		dest:   dest,
		dryRun: true,
		dev:    true,
	})
	if err != nil {
		t.Fatalf("runInstall claude --dev foo: %v", err)
	}
	if !strings.Contains(out.String(), "would symlink claude skill foo") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunInstall_ClaudeWithoutDevRejectsSkillNames(t *testing.T) {
	root := setupE2ERepo(t)
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := runInstall(cmd, root, []string{"claude", "foo"}, installOptions{scope: "user", dryRun: true})
	if err == nil {
		t.Fatal("expected rejection without --dev")
	}
	if !strings.Contains(err.Error(), "without --dev") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveInstallSelections_Multiple(t *testing.T) {
	targets, err := resolveInstallSelections("1,7,5", installOptions{scope: "user"})
	if err != nil {
		t.Fatalf("resolveInstallSelections: %v", err)
	}
	if len(targets) != 3 {
		t.Fatalf("len(targets) = %d, want 3", len(targets))
	}
	if targets[0].agent != "codex" || targets[0].opts.copy {
		t.Fatalf("target[0] = %+v, want codex user symlink", targets[0])
	}
	if targets[1].agent != "droid" || targets[1].opts.scope != "repo" {
		t.Fatalf("target[1] = %+v, want droid repo symlink", targets[1])
	}
	if targets[2].agent != "claude" || !targets[2].opts.dev {
		t.Fatalf("target[2] = %+v, want claude dev", targets[2])
	}
}

func TestResolveInstallSelections_All(t *testing.T) {
	for _, in := range []string{"all", "[all]", " ALL "} {
		targets, err := resolveInstallSelections(in, installOptions{scope: "user"})
		if err != nil {
			t.Fatalf("resolveInstallSelections(%q): %v", in, err)
		}
		if len(targets) != 6 {
			t.Fatalf("resolveInstallSelections(%q) len = %d, want 6", in, len(targets))
		}
		for _, target := range targets {
			if target.opts.copy {
				t.Fatalf("resolveInstallSelections(%q) included conflicting copy target: %+v", in, target)
			}
		}
	}
}

func TestResolveInstallSelections_RejectsConflictingVariants(t *testing.T) {
	for _, in := range []string{"1,3", "6,8", "droid,droid snapshot"} {
		if _, err := resolveInstallSelections(in, installOptions{scope: "user"}); err == nil {
			t.Fatalf("resolveInstallSelections(%q) accepted conflicting variants", in)
		}
	}
}

func TestResolveInstallSelections_RejectsDestWithMultipleTargets(t *testing.T) {
	_, err := resolveInstallSelections("1,7", installOptions{scope: "user", dest: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "--dest requires a single") {
		t.Fatalf("error = %v, want multi-target --dest rejection", err)
	}
}

func TestResolveInstallSelections_DedupesAndPreservesOrder(t *testing.T) {
	targets, err := resolveInstallSelections("5, 1, 5, codex repo", installOptions{scope: "user"})
	if err != nil {
		t.Fatalf("resolveInstallSelections: %v", err)
	}
	if len(targets) != 3 {
		t.Fatalf("len(targets) = %d, want 3 (deduped)", len(targets))
	}
	if targets[0].agent != "claude" || !targets[0].opts.dev {
		t.Fatalf("target[0] = %+v, want claude dev", targets[0])
	}
	if targets[2].agent != "codex" || targets[2].opts.scope != "repo" {
		t.Fatalf("target[2] = %+v, want codex repo", targets[2])
	}
}

func TestResolveInstallSelections_RejectsEmptyAndInvalid(t *testing.T) {
	if _, err := resolveInstallSelections("   ", installOptions{scope: "user"}); err == nil {
		t.Fatal("expected error for empty selection")
	}
	if _, err := resolveInstallSelections("1,99", installOptions{scope: "user"}); err == nil {
		t.Fatal("expected error for invalid token in list")
	}
}

func TestResolveInstallSelection_ClaudeDevPick(t *testing.T) {
	agent, opts, err := resolveInstallSelection("5", installOptions{scope: "user"})
	if err != nil {
		t.Fatalf("resolveInstallSelection: %v", err)
	}
	if agent != "claude" || !opts.dev {
		t.Fatalf("agent=%q dev=%v, want claude/true", agent, opts.dev)
	}
}

func TestRunInstall_RejectsUnknownAgentTypo(t *testing.T) {
	root := setupE2ERepo(t)
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := runInstall(cmd, root, []string{"codeex"}, installOptions{scope: "user"})
	if err == nil {
		t.Fatal("expected error for unknown agent typo")
	}
	if !strings.Contains(err.Error(), "unknown agent or skill") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunInstall_DroidAcceptsSkillNames(t *testing.T) {
	root := setupE2ERepo(t)
	dest := filepath.Join(t.TempDir(), "droid-skills")
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := runInstall(cmd, root, []string{"droid", "foo"}, installOptions{
		dest:   dest,
		dryRun: true,
	})
	if err != nil {
		t.Fatalf("runInstall droid foo: %v", err)
	}
	if !strings.Contains(out.String(), "would symlink droid skill foo") {
		t.Fatalf("output = %q", out.String())
	}
}
