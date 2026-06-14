// Package inventory models Claude assets and produces a review-oriented view by
// merging the skills.toml manifest with a read-only ~/.claude scan. It is
// transport-agnostic: nothing here imports net/http, so a future Wails desktop
// can bind the service layer directly.
package inventory

// AssetType enumerates the kinds of Claude assets vd can surface.
type AssetType string

// Asset type constants, one per supported Claude asset kind.
const (
	Skill   AssetType = "skill"
	Agent   AssetType = "agent"
	Command AssetType = "command"
	Hook    AssetType = "hook"
	Rule    AssetType = "rule"
)

// platformClaude is the only platform today; kept as a field so Cursor can be
// added later without a breaking struct change.
const platformClaude = "claude_code"

// Asset is a single discovered asset on disk (or a hook from settings.json).
type Asset struct {
	Type        AssetType      `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Enabled     bool           `json:"enabled"`
	Path        string         `json:"path"`
	Frontmatter map[string]any `json:"frontmatter,omitempty"`
	Body        string         `json:"-"`
	Platform    string         `json:"platform"`
}
