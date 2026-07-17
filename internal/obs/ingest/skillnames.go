package ingest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SkillRegistry is the set of installed skill names a `$token` must match to be
// counted. Membership is the noise filter: Codex user messages carry `$name`
// tokens that are as often shell variables (`$options`, `$record`) or PHP
// (`$this->x`) as skill invocations, and only the install roots know which names
// are real. An empty registry rejects everything — a missing root is zero skills,
// never an accept-all fallback.
type SkillRegistry map[string]struct{}

func (r SkillRegistry) has(name string) bool {
	_, ok := r[name]
	return ok
}

// codexSkillRe pulls `$name` / `$vd:name` skill tokens out of a Codex user
// message. The `$` must open a token (start of string or after whitespace) so a
// mid-expression `$this->foo` never matches; the `:` in the class carries the
// namespace (`vd:ship`), which canonicalSkill then strips.
var codexSkillRe = regexp.MustCompile(`(?:^|\s)\$([a-z][a-z0-9:_-]{1,40})`)

// matchSkills returns the canonical names of every registered skill invoked in
// msg, in order, repeats included. A nil/empty registry matches nothing.
func (r SkillRegistry) matchSkills(msg string) []string {
	if len(r) == 0 || msg == "" {
		return nil
	}
	var out []string
	for _, m := range codexSkillRe.FindAllStringSubmatch(msg, -1) {
		if name := canonicalSkill(m[1]); name != "" && r.has(name) {
			out = append(out, name)
		}
	}
	return out
}

// canonicalSkill drops a leading namespace (`vd:ship` -> `ship`) and trims the
// separators a token can end on, so it maps to an install-root directory name.
func canonicalSkill(raw string) string {
	if i := strings.LastIndex(raw, ":"); i >= 0 {
		raw = raw[i+1:]
	}
	return strings.Trim(raw, "-_")
}

// LoadSkillRegistry reads every install root once and returns the set of skill
// directory names (those holding a SKILL.md). A root that does not exist is
// skipped silently: the registry is a best-effort filter, not a required input.
func LoadSkillRegistry(roots []string) SkillRegistry {
	reg := SkillRegistry{}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, ".") || !e.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(root, name, "SKILL.md")); err == nil {
				reg[name] = struct{}{}
			}
		}
	}
	return reg
}

// DefaultSkillRoots are the Codex-side install locations vd writes to: the shared
// ~/.agents/skills namespace and ~/.codex/skills. The Claude side already parses
// first-class Skill events, so its roots are not consulted here.
func DefaultSkillRoots() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".agents", "skills"),
		filepath.Join(home, ".codex", "skills"),
	}
}
