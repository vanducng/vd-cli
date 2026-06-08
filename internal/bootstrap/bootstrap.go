// Package bootstrap fetches the skills content repository into a fixed home
// location ($HOME/.vd/skills) so a freshly installed vd binary has skills to
// install from, without the user cloning anything by hand.
package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vanducng/vd-cli/v2/internal/source"
	"github.com/vanducng/vd-cli/v2/internal/updatecheck"
)

const (
	// DefaultRepoURL is the canonical skills content repo cloned on bootstrap.
	DefaultRepoURL = "https://github.com/vanducng/skills.git"
	// RepoEnvVar overrides DefaultRepoURL.
	RepoEnvVar = "VD_SKILLS_REPO"
)

// DefaultRoot returns the standard location for the bootstrapped skills repo:
// $HOME/.vd/skills.
func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".vd", "skills"), nil
}

// RepoURL returns the skills repo URL, honoring VD_SKILLS_REPO.
func RepoURL() string {
	if v := strings.TrimSpace(os.Getenv(RepoEnvVar)); v != "" {
		return v
	}
	return DefaultRepoURL
}

// IsBootstrapped reports whether dir is a populated skills repo (has a manifest).
func IsBootstrapped(dir string) bool {
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "skills.toml"))
	return err == nil
}

// Options controls a bootstrap run.
type Options struct {
	RepoURL string // empty → RepoURL()
	Ref     string // explicit tag/branch; empty → latest release tag
	Update  bool   // move an existing clone to the target ref
	DryRun  bool
}

// Result describes what a bootstrap run did (or would do).
type Result struct {
	Root   string
	Ref    string
	Action string // cloned | updated | already-present | already-current | would-clone | would-update
}

// Run clones the skills repo into DefaultRoot at the latest release tag (or
// opts.Ref). When the home already exists it is a no-op unless opts.Update is
// set, in which case it is moved to the target ref.
func Run(ctx context.Context, opts Options) (Result, error) {
	if err := source.RequireGit(ctx); err != nil {
		return Result{}, err
	}

	root, err := DefaultRoot()
	if err != nil {
		return Result{}, err
	}

	repoURL := opts.RepoURL
	if repoURL == "" {
		repoURL = RepoURL()
	}

	target := opts.Ref
	if target == "" {
		latest, err := LatestReleaseTag(ctx, repoURL)
		if err != nil {
			return Result{}, err
		}
		target = latest
	}

	if !IsBootstrapped(root) {
		if opts.DryRun {
			return Result{Root: root, Ref: target, Action: "would-clone"}, nil
		}
		if err := clone(ctx, repoURL, target, root); err != nil {
			return Result{}, err
		}
		return Result{Root: root, Ref: target, Action: "cloned"}, nil
	}

	current := currentRef(ctx, root)
	if !opts.Update {
		return Result{Root: root, Ref: current, Action: "already-present"}, nil
	}
	if current != "" && current == target {
		return Result{Root: root, Ref: current, Action: "already-current"}, nil
	}
	if opts.DryRun {
		return Result{Root: root, Ref: target, Action: "would-update"}, nil
	}
	if err := moveToRef(ctx, root, target); err != nil {
		return Result{}, err
	}
	return Result{Root: root, Ref: target, Action: "updated"}, nil
}

// LatestReleaseTag returns the highest semver tag advertised by repoURL.
func LatestReleaseTag(ctx context.Context, repoURL string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "ls-remote", "--tags", "--refs", repoURL).Output()
	if err != nil {
		return "", fmt.Errorf("list remote tags for %s: %w", repoURL, err)
	}
	tag := pickLatestTag(strings.Split(string(out), "\n"))
	if tag == "" {
		return "", fmt.Errorf("no semver release tags found at %s", repoURL)
	}
	return tag, nil
}

// pickLatestTag extracts the highest semver tag from `git ls-remote --tags`
// output lines ("<sha>\trefs/tags/<tag>").
func pickLatestTag(lines []string) string {
	best := ""
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		tag := strings.TrimPrefix(fields[1], "refs/tags/")
		if !updatecheck.IsValid(tag) {
			continue
		}
		if best == "" || updatecheck.Less(best, tag) {
			best = tag
		}
	}
	return best
}

func clone(ctx context.Context, url, ref, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("prepare %s: %w", filepath.Dir(dest), err)
	}
	args := []string{"-c", "core.autocrlf=false", "clone", "--depth=1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, url, dest)
	if out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("git clone %s: %w: %s", url, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// moveToRef shallow-fetches ref into an existing clone and force-checks it out.
func moveToRef(ctx context.Context, dir, ref string) error {
	if out, err := exec.CommandContext(ctx, "git", "-C", dir, "fetch", "--depth=1", "--force", "origin",
		"refs/tags/"+ref+":refs/tags/"+ref).CombinedOutput(); err != nil {
		// Fall back to fetching ref as a branch (covers --ref <branch>).
		if out2, err2 := exec.CommandContext(ctx, "git", "-C", dir, "fetch", "--depth=1", "origin", ref).CombinedOutput(); err2 != nil {
			return fmt.Errorf("git fetch %s: %w: %s / %s", ref, err, strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)))
		}
		if out, err := exec.CommandContext(ctx, "git", "-C", dir, "checkout", "-f", "FETCH_HEAD").CombinedOutput(); err != nil {
			return fmt.Errorf("git checkout %s: %w: %s", ref, err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if out, err := exec.CommandContext(ctx, "git", "-C", dir, "checkout", "-f", ref).CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout %s: %w: %s", ref, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// currentRef returns the tag the clone is checked out at, or a short SHA.
func currentRef(ctx context.Context, dir string) string {
	if out, err := exec.CommandContext(ctx, "git", "-C", dir, "describe", "--tags", "--exact-match").Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
