// Package hooks embeds and installs the vd-cli clean-room Claude hooks.
package hooks

import "embed"

//go:embed assets
var FS embed.FS
