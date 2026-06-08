# Contributing to vd

## Dev setup

Requires Go 1.23+ and `make`.

```sh
git clone https://github.com/vanducng/vd-cli.git
cd vd-cli

make build    # compile → ./vd
make test     # go test ./...
make vet      # go vet ./...
make lint     # golangci-lint run (requires golangci-lint in PATH)
```

Install golangci-lint (if missing):
```sh
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

Install goimports (required by the lint formatter):
```sh
go install golang.org/x/tools/cmd/goimports@latest
```

Run a single command without building first:
```sh
make run ARGS="init --force"   # go run ./cmd/vd init --force
```

## Running tests

```sh
make test                                         # all packages
go test ./internal/cli/... -v -run TestAdd        # single test
go test ./... -race -cover                        # with race detector + coverage
```

Integration tests that touch the network or filesystem are in `internal/cli/*_test.go` and `internal/source/*_test.go`. They use temporary Git fixtures and do not require a real upstream connection in the fast path (the fetcher uses a local bare-clone fixture).

The snapshot test in `internal/target/claude_bundle_test.go` is a critical gate: it asserts that the bundle emitter produces byte-equal output to `.claude-plugin/marketplace.json` and `.claude-plugin/plugin.json`. If that test fails after your change, update the snapshots in `internal/target/testdata/bundle-snapshot/` to reflect the intended new output.

## Conventional commits

Use the `vd:` scope for changes to this CLI:

```
feat(vd): add --dry-run flag to vd sync
fix(vd): handle empty skills.lock on first sync
docs(vd): update config-schema.md with version_strategy field
test(vd): add snapshot test for per-skill emitter
chore(vd): bump golangci-lint to v1.62
refactor(vd): extract executor into separate package
```

For changes to the repo's skills (not the CLI), omit the `vd:` scope:
```
feat: add skills/browser-trace from browserbase
```

Do not reference AI tools in commit messages.

## File ownership rules

- CLI source lives in `cmd/vd/` (entrypoint) and `internal/` (packages). Do not restructure these without explicit coordination.
- Test files in `internal/*/` are owned by the same team as the package they test.
- Snapshot data in `internal/target/testdata/` is generated — update it by running the test with `-update` flag (see test file header).

## Release flow

Releases run through a single `release.yml` workflow (mirroring `vanducng/miu-db`
and `vanducng/skills`) that combines [release-please](https://github.com/googleapis/release-please)
and GoReleaser. The workflow authenticates as the **MiuMun GitHub App**, not
`GITHUB_TOKEN` — that is what lets the auto-merged release commit re-trigger the
workflow and lets release-please create the GitHub Release directly (no
`skip-github-release` workaround, no manual tag push).

1. Merge conventional commits to `main` (`feat: ...`, `fix: ...`).
2. release-please opens a PR titled `chore(main): release X.Y.Z` bumping `CHANGELOG.md` and `.release-please-manifest.json`. The workflow auto-merges it (squash).
3. The app-token merge commit re-triggers `release.yml`; release-please then reports `release_created`, cuts the `vX.Y.Z` tag + GitHub Release, and GoReleaser uploads binaries and updates the Homebrew formula in `vanducng/homebrew-tap`.

Required repo secrets: `GH_APP_ID`, `GH_APP_MUNMIU_PRIVATE_KEY`, `HOMEBREW_TAP_GITHUB_TOKEN`.

Release artifacts include:

- `vd_darwin_x86_64.tar.gz`
- `vd_darwin_arm64.tar.gz`
- `vd_linux_x86_64.tar.gz`
- `vd_linux_arm64.tar.gz`
- `vd_windows_x86_64.zip`
- `vd_windows_arm64.zip`

## CI

| Workflow | Trigger | What it does |
|----------|---------|--------------|
| `test.yml` | Push / PR | Go vet, test, lint |
| `release.yml` | Push to `main` / `workflow_dispatch` | release-please (app token) → auto-merge release PR → GoReleaser cross-compile + GitHub Release + Homebrew formula |
| `docs.yml` | Push to `main` / `workflow_dispatch` | Build & deploy the docs site to GitHub Pages |

## Where to file issues

GitHub Issues on `vanducng/vd-cli`.

Include:
- `vd --version` output.
- OS and Go version (`go version`).
- Minimal reproduction steps.
- Relevant section of `skills.toml` (redact personal URLs if needed).
