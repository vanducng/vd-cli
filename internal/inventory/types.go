package inventory

// AssetSummary is a compact, transport-neutral view of one asset.
type AssetSummary struct {
	Type        AssetType `json:"type"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Source      string    `json:"source,omitempty"`
	Mode        string    `json:"mode,omitempty"`
	SHA         string    `json:"sha,omitempty"`
	Drift       string    `json:"drift,omitempty"`
	Enabled     bool      `json:"enabled"`
	Platform    string    `json:"platform"`
}

// Inventory splits assets into manifest-tracked (managed) and discovered.
type Inventory struct {
	Managed    []AssetSummary `json:"managed"`
	Discovered []AssetSummary `json:"discovered"`
}

// SkillDetail is a full view of one skill: summary plus parsed content.
type SkillDetail struct {
	AssetSummary
	Frontmatter map[string]any `json:"frontmatter,omitempty"`
	Body        string         `json:"body"`
	Path        string         `json:"path"`
}

// DoctorEntry mirrors one row of `vd doctor` output.
type DoctorEntry struct {
	Skill  string `json:"skill"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// DoctorReport is the JSON-friendly form of `vd doctor`.
type DoctorReport struct {
	Entries []DoctorEntry `json:"entries"`
}
