package target

import "fmt"

// NewEmitter returns the Emitter for the given target name.
// Known names: "claude", "agents", "droid".
func NewEmitter(name string) (Emitter, error) {
	switch name {
	case "claude":
		return &claudeEmitter{}, nil
	case "agents":
		return &agentsEmitter{}, nil
	case "droid":
		return &droidEmitter{}, nil
	default:
		return nil, fmt.Errorf("unknown emitter %q: valid names are claude, agents, droid", name)
	}
}
