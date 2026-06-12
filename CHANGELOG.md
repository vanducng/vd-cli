# Changelog

All notable changes to the `vd` CLI.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), versions follow [SemVer](https://semver.org/).

## [2.7.0](https://github.com/vanducng/vd-cli/compare/v2.6.0...v2.7.0) (2026-06-12)


### Features

* **hooks:** worktree-aware artifact paths; drop .ck.json fallback ([#16](https://github.com/vanducng/vd-cli/issues/16)) ([4c06505](https://github.com/vanducng/vd-cli/commit/4c065050d1eb71a242055f94c27b385d49b4142a))

## [2.6.0](https://github.com/vanducng/vd-cli/compare/v2.5.0...v2.6.0) (2026-06-10)


### Features

* **install:** add install.sh for macOS/Linux ([afeed5d](https://github.com/vanducng/vd-cli/commit/afeed5d42521fd7b5165852c0a86ea9e8b4af436))


### Bug Fixes

* **upgrade:** detect Homebrew install before download and add brew trust hint ([fcfa2fc](https://github.com/vanducng/vd-cli/commit/fcfa2fc568b359d7de6b3dfe328e102073bcbdec))

## [2.5.0](https://github.com/vanducng/vd-cli/compare/v2.4.0...v2.5.0) (2026-06-08)


### Features

* **hooks:** statusline, scout-block, team auxiliaries (clean-room) ([#11](https://github.com/vanducng/vd-cli/issues/11)) ([78e81f1](https://github.com/vanducng/vd-cli/commit/78e81f119f78008292be217943dcaffc2f217bf2))

## [2.4.0](https://github.com/vanducng/vd-cli/compare/v2.3.0...v2.4.0) (2026-06-08)


### Features

* own the Claude Code control plane (clean-room hooks + .work umbrella) ([#9](https://github.com/vanducng/vd-cli/issues/9)) ([ec57537](https://github.com/vanducng/vd-cli/commit/ec5753749f029b899855cc2c2b9ca0e1ab408599))

## [2.3.0](https://github.com/vanducng/vd-cli/compare/v2.2.0...v2.3.0) (2026-06-08)


### Features

* add 'vd upgrade' self-upgrade command ([f8bc629](https://github.com/vanducng/vd-cli/commit/f8bc6295e1fe98d8f5b734f89c9e4df4c0fbaf17))
* **install:** multi-select picker and bootstrap skills into ~/.vd/skills ([95ac5fc](https://github.com/vanducng/vd-cli/commit/95ac5fc354fa1f10da9d21d3774e0ef80be0e6f5))

## [2.2.0](https://github.com/vanducng/vd-cli/compare/v2.1.0...v2.2.0) (2026-05-24)


### Features

* **release:** add Windows ARM64 artifacts ([#3](https://github.com/vanducng/vd-cli/issues/3))

## [2.1.0](https://github.com/vanducng/vd-cli/compare/v2.0.0...v2.1.0) (2026-05-24)


### Features

* **install:** add `claude --dev` for per-skill symlinks ([#2](https://github.com/vanducng/vd-cli/issues/2)) ([8fd2743](https://github.com/vanducng/vd-cli/commit/8fd27435f88225fc7ba1a97c91bb70f674ed845c))


### Bug Fixes

* **module:** add /v2 suffix to module path for Go SIV compliance ([216aba8](https://github.com/vanducng/vd-cli/commit/216aba8d949747731707216505ebad43f1055728))

## [2.0.1] (2026-05-23)

### Bug Fixes

* **module:** add required `/v2` suffix to module path per Go semantic-import-versioning. `go install github.com/vanducng/vd-cli/v2/cmd/vd@v2.0.1` now works. Homebrew install unchanged. v2.0.0 is `go install`-broken — skip it.

## [2.0.0] (2026-05-23)

### BREAKING

* Module path renamed from `github.com/vanducng/skills/tools/vd` to `github.com/vanducng/vd-cli`. `go install` callers must update their import path. Homebrew (`brew install vanducng/tap/vd`) and prebuilt release binaries are unaffected.

### Migration

* Extracted from [`vanducng/skills`](https://github.com/vanducng/skills) monorepo (`tools/vd/` subdirectory) at commit `2bda3e8`. Pre-v2.0.0 history below references commits in that repo.

---

## [1.1.0](https://github.com/vanducng/skills/compare/v1.0.0...v1.1.0) (2026-05-05)


### Features

* **vd:** passive upstream version check with one-line stderr nudge ([db883de](https://github.com/vanducng/skills/commit/db883deaa589c2def5494fdf7a5ac9bf377fb2e5))
* **vd:** support VD_ROOT env var for repo root resolution ([8308e06](https://github.com/vanducng/skills/commit/8308e069f3739d423dfc6bdccd08d4aadc13bef1))


### Bug Fixes

* **ci:** skip github-release in release-please; document manual tag step ([05d3e87](https://github.com/vanducng/skills/commit/05d3e8703cfc12650dd5e4f5a4bd36a8fc34c61f))

## 1.0.0 (2026-05-05)


### Features

* **vd:** add CLI for vendoring + syncing skills ([#10](https://github.com/vanducng/skills/issues/10)) ([800da42](https://github.com/vanducng/skills/commit/800da420f110381ef310fdea72794ce5ce0423e2))

## [Unreleased]

### Added
- `VD_ROOT` env var as a repo-root source (between the `--root` flag and the `.git` walk-up). Both override sources validate that the path exists and is a directory; invalid values fail fast.
- Passive upstream version check on every command — prints a one-line stderr nudge when a newer release is available. Cached 24h under `$XDG_CACHE_HOME/vd/version-check.json`. Gated by `VD_NO_UPDATE_CHECK`, `CI`, `--quiet`, `dev` builds, and non-TTY stderr. No telemetry, no opt-in, fully offline-safe.
- Initial CLI: `init`, `add`, `list`, `sync`, `update`, `diff`, `doctor`, `pin`, `detach`, `remove`, `build`, `cache clean`.
- Bundle and per-skill emitter modes for `marketplace.json` and `plugin.json`.
- `.agents/` symlink emitter (relative symlinks; Windows falls back to directory copy).
- `skills.toml` manifest schema (version 1) with `[meta]`, `[sources.*]`, `[skills.*]`, `[targets.claude]`, `[targets.claude.bundle]`, `[plugin.*]` blocks.
- `skills.lock` for reproducible installs (SHA + TreeHash per skill).
- Dirty-check ("refuse-on-dirty"): `vd sync` refuses to overwrite locally modified skills without `--force`.
- Bundle emitter seeds defaults from live `marketplace.json` so first-run output is byte-equal to the existing file.
- GoReleaser monorepo distribution with tag pattern `v*`.
- GitHub Actions path-filtered CI: `vd-test.yml` (test + lint on `tools/vd/**` changes), `vd-release.yml` (GoReleaser on `v*` tags), `vd-release-please.yml` (automated release PR).
- Dogfood: `browserbase/skills/browser` and `browserbase/skills/browser-trace` onboarded via `vd add` + `vd sync` with zero diff on `.claude-plugin/`.
