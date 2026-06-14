# vd desktop (Wails)

Native desktop frontend for vd — a sibling of `vd web` and `vd tui`. It renders the
same inventory (managed skills with drift, discovered `~/.claude` assets, hooks) in a
native window instead of a browser tab.

It is a **separate Go module** so Wails' CGO/WebView dependencies never touch the
pure-Go `vd` CLI. It reuses the parent module's `internal/inventory` backend and
`internal/ui/web` handler via a local `replace` directive:

- **Assets** — the embedded React SPA (`internal/ui/web`), served by Wails.
- **`/api/*`** — routed to the same read-only `web.Server` handler used by `vd web`.

No bound methods, no second frontend: the desktop is a thin native shell over the
already-verified web stack.

## Prerequisites

- Go 1.23+
- A C toolchain (Xcode CLT on macOS) — Wails uses CGO + the system WebView
- The Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

## Build & run

```sh
# from repo root
make desktop          # → desktop/build/bin/vd-desktop.app
make desktop-dev      # live-reload dev mode

# or directly
cd desktop
wails build -s        # -s skips frontend tooling; assets come from Go
wails dev
```

The app resolves the vd repo for managed skills via `VD_ROOT`, else the nearest
`.git` ancestor of the working directory, else `~/.vd/skills`. Discovered `~/.claude`
assets always appear regardless of repo.

## Why a separate module

Wails requires Go 1.24+ for its newest releases and pulls a large CGO dependency tree.
Isolating it here keeps `go build ./...`, the test suite, and the single-binary release
of the `vd` CLI pure-Go and unaffected.
