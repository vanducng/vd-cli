// Package extension discovers and validates vd-managed MCP servers/services
// declared as extensions/<name>/extension.toml. vd is the manager (it registers
// these into Codex and Claude); it does not host them. See docs/decisions/0001.
package extension

import (
	"bytes"
	"fmt"
	"os"

	toml "github.com/pelletier/go-toml/v2"
)

// Extension is one MCP server/service parsed from an extension.toml.
type Extension struct {
	Name              string   `toml:"name"`
	Description       string   `toml:"description"`
	Transport         string   `toml:"transport"` // stdio | http
	Command           string   `toml:"command"`   // stdio
	Args              []string `toml:"args"`      // stdio; a "{dir}" token resolves to Dir at registration
	URL               string   `toml:"url"`       // http
	Env               []string `toml:"env"`       // var NAMES only — preflight/doctor; never written to config
	Targets           []string `toml:"targets"`   // codex | claude
	Scope             string   `toml:"scope"`     // default registration scope: project | user | global
	StartupTimeoutSec int      `toml:"startup_timeout_sec"`
	Enabled           bool     `toml:"enabled"`

	// Dir is the absolute extension directory, set at discovery (not from TOML).
	Dir string `toml:"-"`
}

var (
	validTransports = map[string]bool{"stdio": true, "http": true}
	validTargets    = map[string]bool{"codex": true, "claude": true}
	validScopes     = map[string]bool{"project": true, "user": true, "global": true}
)

// LoadExtension parses and validates an extension.toml. Unknown fields error
// (a typo shouldn't silently vanish). Scope defaults to "project" when omitted.
func LoadExtension(path string) (Extension, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Extension{}, fmt.Errorf("extension manifest not found: %s", path)
		}
		return Extension{}, fmt.Errorf("read extension manifest %s: %w", path, err)
	}

	var e Extension
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&e); err != nil {
		return Extension{}, fmt.Errorf("parse extension manifest %s: %w", path, err)
	}
	if e.Scope == "" {
		e.Scope = "project"
	}
	if err := e.validate(); err != nil {
		return Extension{}, fmt.Errorf("%s: %w", path, err)
	}
	return e, nil
}

func (e Extension) validate() error {
	if e.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !validTransports[e.Transport] {
		return fmt.Errorf("transport %q must be stdio or http", e.Transport)
	}
	switch e.Transport {
	case "stdio":
		if e.Command == "" {
			return fmt.Errorf("stdio transport requires command")
		}
	case "http":
		if e.URL == "" {
			return fmt.Errorf("http transport requires url")
		}
	}
	if len(e.Targets) == 0 {
		return fmt.Errorf("at least one target is required (codex and/or claude)")
	}
	for _, t := range e.Targets {
		if !validTargets[t] {
			return fmt.Errorf("unknown target %q (want codex or claude)", t)
		}
	}
	if !validScopes[e.Scope] {
		return fmt.Errorf("scope %q must be project, user, or global", e.Scope)
	}
	return nil
}

// ResolvedArgs returns Args with the "{dir}" token replaced by the extension's
// absolute directory, so the launch command is runnable from anywhere.
func (e Extension) ResolvedArgs() []string {
	out := make([]string, len(e.Args))
	for i, a := range e.Args {
		if a == "{dir}" {
			out[i] = e.Dir
		} else {
			out[i] = a
		}
	}
	return out
}
