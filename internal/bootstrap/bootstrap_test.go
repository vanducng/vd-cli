package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPickLatestTag(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{
			name: "highest semver wins regardless of order",
			lines: []string{
				"abc123\trefs/tags/v1.0.0",
				"def456\trefs/tags/v1.2.0",
				"ghi789\trefs/tags/v1.1.5",
			},
			want: "v1.2.0",
		},
		{
			name: "skips non-semver and pre-release tags",
			lines: []string{
				"a\trefs/tags/latest",
				"b\trefs/tags/v2.0.0-rc1",
				"c\trefs/tags/v0.9.0",
			},
			want: "v0.9.0",
		},
		{
			name:  "no semver tags yields empty",
			lines: []string{"a\trefs/tags/nightly", "garbage line"},
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickLatestTag(tt.lines); got != tt.want {
				t.Fatalf("pickLatestTag = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRepoURL_EnvOverride(t *testing.T) {
	t.Setenv(RepoEnvVar, "")
	if got := RepoURL(); got != DefaultRepoURL {
		t.Fatalf("RepoURL = %q, want default %q", got, DefaultRepoURL)
	}
	t.Setenv(RepoEnvVar, "https://example.com/custom.git")
	if got := RepoURL(); got != "https://example.com/custom.git" {
		t.Fatalf("RepoURL = %q, want override", got)
	}
}

func TestIsBootstrapped(t *testing.T) {
	dir := t.TempDir()
	if IsBootstrapped(dir) {
		t.Fatal("empty dir should not be bootstrapped")
	}
	if IsBootstrapped("") {
		t.Fatal("empty path should not be bootstrapped")
	}
	if err := os.WriteFile(filepath.Join(dir, "skills.toml"), []byte("[meta]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsBootstrapped(dir) {
		t.Fatal("dir with skills.toml should be bootstrapped")
	}
}

func TestDefaultRoot(t *testing.T) {
	root, err := DefaultRoot()
	if err != nil {
		t.Fatalf("DefaultRoot: %v", err)
	}
	if filepath.Base(root) != "skills" || filepath.Base(filepath.Dir(root)) != ".vd" {
		t.Fatalf("DefaultRoot = %q, want .../.vd/skills", root)
	}
}
