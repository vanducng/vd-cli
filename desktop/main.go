// Command vd-desktop is the Wails desktop frontend for vd. It is a sibling of
// `vd web` and `vd tui` under the internal/ui umbrella: a native window that
// reuses the exact same internal/inventory backend and internal/ui/web handler
// (embedded React SPA + read-only JSON API). It lives in its own module so
// Wails' CGO/WebView dependency never touches the pure-Go `vd` CLI.
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/vanducng/vd-cli/v2/internal/inventory"
	webui "github.com/vanducng/vd-cli/v2/internal/ui/web"
)

func main() {
	claudeHome, err := claudeDir()
	if err != nil {
		log.Fatal(err)
	}
	srv, err := webui.NewServer(inventory.NewService(repoRoot(), claudeHome))
	if err != nil {
		log.Fatal(err)
	}
	assets, err := webui.StaticFS()
	if err != nil {
		log.Fatal(err)
	}

	// Assets serve the embedded SPA; Handler serves /api/* and SPA fallback routes.
	err = wails.Run(&options.App{
		Title:  "vd desktop",
		Width:  1200,
		Height: 820,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: srv.Handler(),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// repoRoot resolves the vd repo whose skills.toml drives the managed inventory.
// Order: VD_ROOT → nearest .git ancestor of CWD → ~/.vd/skills → CWD.
func repoRoot() string {
	if r := os.Getenv("VD_ROOT"); r != "" {
		return r
	}
	if dir, err := os.Getwd(); err == nil {
		for {
			if fi, err := os.Stat(filepath.Join(dir, ".git")); err == nil && fi.IsDir() {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if vd := filepath.Join(home, ".vd", "skills"); dirExists(vd) {
			return vd
		}
	}
	wd, _ := os.Getwd()
	return wd
}

func claudeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
