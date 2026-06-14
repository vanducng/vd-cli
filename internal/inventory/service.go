package inventory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/vanducng/vd-cli/v2/internal/config"
	vdsync "github.com/vanducng/vd-cli/v2/internal/sync"
)

// ErrNotFound is returned when a requested asset does not exist.
var ErrNotFound = errors.New("not found")

// Service produces inventory views from the repo manifest and a read-only
// ~/.claude scan. Transport-agnostic — safe to bind directly from Wails.
type Service struct {
	RepoRoot   string
	ClaudeHome string // the .claude directory
}

// NewService constructs a Service.
func NewService(repoRoot, claudeHome string) *Service {
	return &Service{RepoRoot: repoRoot, ClaudeHome: claudeHome}
}

func (s *Service) manifestPath() string { return filepath.Join(s.RepoRoot, "skills.toml") }
func (s *Service) lockPath() string     { return filepath.Join(s.RepoRoot, "skills.lock") }
func (s *Service) skillsDir() string    { return filepath.Join(s.RepoRoot, "skills") }
func (s *Service) settingsPath() string { return filepath.Join(s.ClaudeHome, "settings.json") }

// Inventory merges manifest-tracked skills (with drift) and discovered ~/.claude assets.
func (s *Service) Inventory() (*Inventory, error) {
	inv := &Inventory{Managed: []AssetSummary{}, Discovered: []AssetSummary{}}

	manifest, err := config.Load(s.manifestPath())
	if err != nil {
		return nil, fmt.Errorf("load skills.toml: %w", err)
	}
	lock, err := config.LoadLock(s.lockPath())
	if err != nil {
		return nil, fmt.Errorf("load skills.lock: %w", err)
	}

	names := sortedKeys(manifest.Skills)
	managed := map[string]bool{}
	for _, name := range names {
		sc := manifest.Skills[name]
		drift, sha := s.skillDrift(name, lock)
		inv.Managed = append(inv.Managed, AssetSummary{
			Type: Skill, Name: name, Description: s.repoSkillDescription(name),
			Source: sc.Source, Mode: defMode(sc.Mode), SHA: sha,
			Drift: drift.String(), Enabled: true, Platform: platformClaude,
		})
		managed[name] = true
	}

	scanned, err := NewClaudeAdapter(s.ClaudeHome).Scan()
	if err != nil {
		return nil, fmt.Errorf("scan ~/.claude: %w", err)
	}
	for _, a := range scanned {
		if a.Type == Skill && managed[a.Name] {
			continue // already shown as managed
		}
		inv.Discovered = append(inv.Discovered, AssetSummary{
			Type: a.Type, Name: a.Name, Description: a.Description,
			Enabled: a.Enabled, Platform: a.Platform,
		})
	}
	return inv, nil
}

// SkillDetail returns the full view of a skill: repo-managed copy preferred,
// else a discovered ~/.claude skill.
func (s *Service) SkillDetail(name string) (*SkillDetail, error) {
	if repoPath := filepath.Join(s.skillsDir(), name, "SKILL.md"); fileExists(repoPath) {
		return s.detail(name, repoPath, true, true)
	}
	if claudePath, enabled := skillMarker(filepath.Join(s.ClaudeHome, "skills", name)); claudePath != "" {
		return s.detail(name, claudePath, false, enabled)
	}
	return nil, fmt.Errorf("skill %q: %w", name, ErrNotFound)
}

// Hooks returns the Claude hooks registered in settings.json.
func (s *Service) Hooks() ([]Asset, error) {
	return ReadHooks(s.settingsPath())
}

// Doctor reproduces `vd doctor` as structured data using the same sync helpers.
func (s *Service) Doctor() (*DoctorReport, error) {
	lock, err := config.LoadLock(s.lockPath())
	if err != nil {
		return nil, fmt.Errorf("load skills.lock: %w", err)
	}
	rep := &DoctorReport{Entries: []DoctorEntry{}}
	for _, name := range sortedLockKeys(lock.Skills) {
		entry := lock.Skills[name]
		dir := filepath.Join(s.skillsDir(), name)
		fsSHA := treeHashIfExists(dir)
		drift := vdsync.ComputeDrift(name, lockRef(entry), fsSHA)
		detail := ""
		switch drift {
		case vdsync.DriftLocal:
			detail = fmt.Sprintf("lock=%s fs=%s", short(entry.SHA), short(fsSHA))
		case vdsync.DriftMissing:
			detail = fmt.Sprintf("expected at %s", dir)
		}
		rep.Entries = append(rep.Entries, DoctorEntry{Skill: name, Status: drift.String(), Detail: detail})
	}
	// FS dirs absent from the lock → untracked (hand-authored / detached).
	entries, _ := os.ReadDir(s.skillsDir())
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, ok := lock.Skills[e.Name()]; !ok {
			rep.Entries = append(rep.Entries, DoctorEntry{
				Skill: e.Name(), Status: "untracked", Detail: "(hand-authored or detached — OK)",
			})
		}
	}
	return rep, nil
}

// skillDrift computes drift + short SHA for a manifest skill, mirroring `vd doctor`.
func (s *Service) skillDrift(name string, lock *config.Lockfile) (vdsync.DriftStatus, string) {
	entry, hasLock := lock.Skills[name]
	fsSHA := treeHashIfExists(filepath.Join(s.skillsDir(), name))
	if !hasLock {
		return vdsync.ComputeDrift(name, "", fsSHA), ""
	}
	return vdsync.ComputeDrift(name, lockRef(entry), fsSHA), short(entry.SHA)
}

func (s *Service) detail(name, path string, managed, enabled bool) (*SkillDetail, error) {
	data, err := readCapped(path, maxAssetBytes)
	if err != nil {
		return nil, err
	}
	fm, body, err := ParseFrontmatter(data)
	if err != nil {
		return nil, err
	}
	sum := AssetSummary{
		Type: Skill, Name: name, Description: Describe(fm),
		Enabled: enabled, Platform: platformClaude,
	}
	if managed {
		if lock, err := config.LoadLock(s.lockPath()); err == nil {
			drift, sha := s.skillDrift(name, lock)
			sum.Drift, sum.SHA = drift.String(), sha
		}
		if manifest, err := config.Load(s.manifestPath()); err == nil {
			if sc, ok := manifest.Skills[name]; ok {
				sum.Source, sum.Mode = sc.Source, defMode(sc.Mode)
			}
		}
	}
	return &SkillDetail{AssetSummary: sum, Frontmatter: fm, Body: body, Path: path}, nil
}

func (s *Service) repoSkillDescription(name string) string {
	data, err := readCapped(filepath.Join(s.skillsDir(), name, "SKILL.md"), maxAssetBytes)
	if err != nil {
		return ""
	}
	fm, _, err := ParseFrontmatter(data)
	if err != nil {
		return ""
	}
	return Describe(fm)
}

func treeHashIfExists(dir string) string {
	if _, err := os.Stat(dir); err != nil {
		return ""
	}
	h, _ := vdsync.TreeHash(dir)
	return h
}

// lockRef prefers the tree hash for dirty detection, falling back to the SHA.
func lockRef(e config.LockEntry) string {
	if e.TreeHash != "" {
		return e.TreeHash
	}
	return e.SHA
}

func defMode(m string) string {
	if m == "" {
		return "tracked"
	}
	return m
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func sortedKeys(m map[string]config.SkillConfig) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedLockKeys(m map[string]config.LockEntry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
