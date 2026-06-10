// Package hooks embeds and installs the vd-cli clean-room Claude hooks.
package hooks

import "embed"

// FS holds the embedded hook assets.
//
//go:embed assets
var FS embed.FS
