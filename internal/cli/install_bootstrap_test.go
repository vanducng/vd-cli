package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveInstallRoot_RootFlagWins(t *testing.T) {
	root := setupE2ERepo(t)
	cmd := &cobra.Command{}
	got, err := resolveInstallRoot(cmd, root, false)
	if err != nil {
		t.Fatalf("resolveInstallRoot: %v", err)
	}
	if got != root {
		t.Fatalf("root = %q, want %q", got, root)
	}
}

func TestResolveInstallRoot_VDRootEnv(t *testing.T) {
	root := setupE2ERepo(t)
	t.Setenv(rootEnvVar, root)
	cmd := &cobra.Command{}
	got, err := resolveInstallRoot(cmd, "", false)
	if err != nil {
		t.Fatalf("resolveInstallRoot: %v", err)
	}
	if got != root {
		t.Fatalf("root = %q, want %q", got, root)
	}
}

func TestResolveInstallRoot_NoSkillsNonInteractiveErrors(t *testing.T) {
	// Isolate HOME so ~/.vd/skills is guaranteed absent, and clear VD_ROOT.
	t.Setenv("HOME", t.TempDir())
	t.Setenv(rootEnvVar, "")

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetIn(&bytes.Buffer{}) // non-*os.File stdin → non-interactive

	_, err := resolveInstallRoot(cmd, "", false)
	if err == nil {
		t.Fatal("expected error when no skills are set up")
	}
	if !strings.Contains(err.Error(), "vd bootstrap") {
		t.Fatalf("error should hint at bootstrap, got: %v", err)
	}
}

func TestResolveInstallRoot_DryRunWithoutSkillsErrors(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(rootEnvVar, "")

	cmd := &cobra.Command{}
	_, err := resolveInstallRoot(cmd, "", true)
	if err == nil || !strings.Contains(err.Error(), "vd bootstrap") {
		t.Fatalf("expected bootstrap hint on dry-run without skills, got: %v", err)
	}
}
