package inventory

import (
	"sort"

	"github.com/vanducng/vd-cli/v2/internal/claudeconfig"
)

// ReadHooks reads settings.json at the given path and returns one Asset per
// registered hook command, flagging vd-managed ones. Missing file → empty.
func ReadHooks(settingsPath string) ([]Asset, error) {
	s, err := claudeconfig.ReadSettingsAt(settingsPath)
	if err != nil {
		return nil, err
	}
	var out []Asset
	for event, entries := range s.Hooks {
		for _, e := range entries {
			for _, item := range e.Hooks {
				name := event
				if e.Matcher != "" {
					name = event + ":" + e.Matcher
				}
				out = append(out, Asset{
					Type:        Hook,
					Name:        name,
					Description: item.Command,
					Enabled:     true,
					Path:        settingsPath,
					Frontmatter: map[string]any{
						"event":       event,
						"matcher":     e.Matcher,
						"managedByVd": claudeconfig.IsManagedCommand(item.Command),
					},
					Platform: platformClaude,
				})
			}
		}
	}
	// Deterministic order: map iteration above is unordered.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Description < out[j].Description
	})
	return out, nil
}
