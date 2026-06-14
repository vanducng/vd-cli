// Package web serves a localhost-only, read-only web view of the inventory
// produced by internal/inventory. It is the HTTP frontend under the internal/ui
// umbrella; sibling tui/ and desktop/ (Wails) frontends can bind the same
// internal/inventory service without this package.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var staticAssets embed.FS

// staticFS returns the embedded SPA files rooted at the static/ dir.
func staticFS() (fs.FS, error) {
	return fs.Sub(staticAssets, "static")
}

// StaticFS exposes the embedded SPA for non-HTTP frontends (e.g. the Wails
// desktop), which serve the static assets directly and route /api/* to Handler.
func StaticFS() (fs.FS, error) {
	return staticFS()
}
