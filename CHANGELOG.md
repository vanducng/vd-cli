# Changelog

All notable changes to the `vd` CLI.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), versions follow [SemVer](https://semver.org/).

## [3.11.0](https://github.com/vanducng/vd-cli/compare/v3.10.0...v3.11.0) (2026-07-17)


### Features

* **obs:** skill-level error observability (closes [#71](https://github.com/vanducng/vd-cli/issues/71)) ([#74](https://github.com/vanducng/vd-cli/issues/74)) ([1496781](https://github.com/vanducng/vd-cli/commit/14967812a592b599fc6b06de512cc6b93b0bba4c))

## [3.10.0](https://github.com/vanducng/vd-cli/compare/v3.9.0...v3.10.0) (2026-07-17)


### Features

* **web:** redesign portal into an agent-management console — judged 9.5/10 ([#72](https://github.com/vanducng/vd-cli/issues/72)) ([0a5405e](https://github.com/vanducng/vd-cli/commit/0a5405e219fde0ee4749694509dc6a0309e9c634))

## [3.9.0](https://github.com/vanducng/vd-cli/compare/v3.8.0...v3.9.0) (2026-07-17)


### Features

* **web:** vd obs web portal — sessions, transcript, usage on the fastreact SPA ([#68](https://github.com/vanducng/vd-cli/issues/68)) ([97c73a9](https://github.com/vanducng/vd-cli/commit/97c73a95befd91c06aef6099ce96870e78ebcd09))

## [3.8.0](https://github.com/vanducng/vd-cli/compare/v3.7.0...v3.8.0) (2026-07-17)


### Features

* **obs:** vd obs — local Claude Code + Codex session observability ([#66](https://github.com/vanducng/vd-cli/issues/66)) ([20c4fe0](https://github.com/vanducng/vd-cli/commit/20c4fe0cd2e6abe6634d5d9c3d1ad4dad0bca248))

## [3.7.0](https://github.com/vanducng/vd-cli/compare/v3.6.2...v3.7.0) (2026-07-10)


### Features

* **hooks:** accepted uv runtime and dropped the bundled Node hook set ([#63](https://github.com/vanducng/vd-cli/issues/63)) ([98d7b9c](https://github.com/vanducng/vd-cli/commit/98d7b9c1d6f342e7dbcec4786c7a29939901bb01))

## [3.6.2](https://github.com/vanducng/vd-cli/compare/v3.6.1...v3.6.2) (2026-06-28)


### Bug Fixes

* **hooks:** anchor umbrella at cwd when no git root ([#60](https://github.com/vanducng/vd-cli/issues/60)) ([b70f4a8](https://github.com/vanducng/vd-cli/commit/b70f4a877aa7c1df5474982bad5601f307e7ee1a))

## [3.6.1](https://github.com/vanducng/vd-cli/compare/v3.6.0...v3.6.1) (2026-06-25)


### Bug Fixes

* **codex-workflow:** suppress noisy mcp notification-validation warning ([#57](https://github.com/vanducng/vd-cli/issues/57)) ([2c1d4dc](https://github.com/vanducng/vd-cli/commit/2c1d4dcb7d9fd50dba8dac835a9786f88132e34f))

## [3.6.0](https://github.com/vanducng/vd-cli/compare/v3.5.0...v3.6.0) (2026-06-25)


### Features

* **mcp:** 'vd mcp logs' lists available logs when no name given ([#55](https://github.com/vanducng/vd-cli/issues/55)) ([d40f58a](https://github.com/vanducng/vd-cli/commit/d40f58a9fa427598356d3cf26845d3b62ccb2073))

## [3.5.0](https://github.com/vanducng/vd-cli/compare/v3.4.1...v3.5.0) (2026-06-25)


### Features

* **codex-workflow:** selective MCP passthrough + logs --follow ([#53](https://github.com/vanducng/vd-cli/issues/53)) ([65b42f7](https://github.com/vanducng/vd-cli/commit/65b42f790bea422b0c21a1bb165ac128f2bdb764))

## [3.4.1](https://github.com/vanducng/vd-cli/compare/v3.4.0...v3.4.1) (2026-06-25)


### Bug Fixes

* **codex-workflow:** kill recursive spawn + transparent extension logs ([#51](https://github.com/vanducng/vd-cli/issues/51)) ([d4bf998](https://github.com/vanducng/vd-cli/commit/d4bf9980bde46c189b82414c03aa447e531f5ae0))

## [3.4.0](https://github.com/vanducng/vd-cli/compare/v3.3.0...v3.4.0) (2026-06-25)


### Features

* Codex dynamic-workflow capability — MCP extensions + run_workflow ([#49](https://github.com/vanducng/vd-cli/issues/49)) ([a015861](https://github.com/vanducng/vd-cli/commit/a015861cbc538cfa7d5ce1d85d3f9714b0e5ddb5))

## [3.3.0](https://github.com/vanducng/vd-cli/compare/v3.2.1...v3.3.0) (2026-06-21)


### Features

* **hooks:** added Codex context hook support ([#47](https://github.com/vanducng/vd-cli/issues/47)) ([9544e02](https://github.com/vanducng/vd-cli/commit/9544e020ab0533a2695a1d98f2af37b4827407ec))

## [3.2.1](https://github.com/vanducng/vd-cli/compare/v3.2.0...v3.2.1) (2026-06-20)


### Bug Fixes

* **hooks:** align home guard path comparisons ([698b62b](https://github.com/vanducng/vd-cli/commit/698b62be28163185e3148808a923b37db222cfee))
* **hooks:** avoid repeated realpath calls in home guard ([9e6a2ee](https://github.com/vanducng/vd-cli/commit/9e6a2ee1c7730ff9fc6b1d6580b28a7285093f7e))
* **hooks:** bound home boundary traversal ([2664819](https://github.com/vanducng/vd-cli/commit/266481960f14c34523d8e4070cb179a840b43af5))
* **hooks:** canonicalize home guard fallback ([770639d](https://github.com/vanducng/vd-cli/commit/770639d2103d86b5626970e46e28c098a539e98d))
* **hooks:** centralize path case handling ([5bd7dff](https://github.com/vanducng/vd-cli/commit/5bd7dffeb82aecea3c045763ded33302c1919f36))
* **hooks:** clarify home path comparison contract ([5cf2777](https://github.com/vanducng/vd-cli/commit/5cf2777fd3181de7af040bb888ebe12a61136d1d))
* **hooks:** compare home scan paths case-insensitively on windows ([338df4d](https://github.com/vanducng/vd-cli/commit/338df4d0704582a3f68f483944fe7d496655e359))
* **hooks:** compare resolved git root fallback ([e5434d2](https://github.com/vanducng/vd-cli/commit/e5434d2c85418efee7ea91ab96fb2168714b4025))
* **hooks:** constrain home boundary scan ([f768727](https://github.com/vanducng/vd-cli/commit/f7687271212a053f91e33020679b54af2e785650))
* **hooks:** document home boundary path assumptions ([cdd4dcb](https://github.com/vanducng/vd-cli/commit/cdd4dcb8fa197890a983b51f2c7100488177cb12))
* **hooks:** guard umbrella against a stray $HOME ancestor repo ([b300b46](https://github.com/vanducng/vd-cli/commit/b300b469011bf1f4188c2e5118259c80020f4832))
* **hooks:** guard umbrella against a stray $HOME ancestor repo ([b65bdb9](https://github.com/vanducng/vd-cli/commit/b65bdb9c94309138aa93ee71547e731a8e602042))
* **hooks:** harden home boundary containment ([0997b1c](https://github.com/vanducng/vd-cli/commit/0997b1cfacd5c37b57e7b6d63c2012a2816c61d0))
* **hooks:** harden home path equality checks ([1a72844](https://github.com/vanducng/vd-cli/commit/1a72844aa4dfefc1ac297ead02e05fdb8ffc0481))
* **hooks:** normalize git helper cache keys ([356748a](https://github.com/vanducng/vd-cli/commit/356748a8378b21a02c6957532a62da435f890642))
* **hooks:** normalize home boundary inputs ([3cff898](https://github.com/vanducng/vd-cli/commit/3cff898e9c094753c1cf8c48e8f5a35e0ebc8e8e))
* **hooks:** normalize home guard fallback paths ([dd6a57f](https://github.com/vanducng/vd-cli/commit/dd6a57fdc42e76e7c7205ee169488862ed9c8cfc))
* **hooks:** normalize home path separators ([4ad6609](https://github.com/vanducng/vd-cli/commit/4ad6609d92120b487b8f95ffc34cc475543bf387))
* **hooks:** normalize home relative inputs ([11dccf3](https://github.com/vanducng/vd-cli/commit/11dccf366a654699ab726244275f915f7e6e7cbe))
* **hooks:** normalize umbrella git base dir ([ea1edba](https://github.com/vanducng/vd-cli/commit/ea1edba98bafe028cb38fc9b6c1bf07b38e357a1))
* **hooks:** normalize umbrella git root ([197f455](https://github.com/vanducng/vd-cli/commit/197f4554c009e143e03d4ac6e4785d7cecbc1015))
* **hooks:** preserve nested git anchor under home guard ([c03ff17](https://github.com/vanducng/vd-cli/commit/c03ff1768ca71574abcf1ce18708a47d9a8b3d88))
* **hooks:** reuse canonical base in home scan ([ebf2bc3](https://github.com/vanducng/vd-cli/commit/ebf2bc31290027007a6be48129fc2bfb222f047a))
* **hooks:** simplify git root home comparison ([0dde531](https://github.com/vanducng/vd-cli/commit/0dde53121e2a77c4eebcc5d4e7a41641c2d4bb05))
* **hooks:** simplify home realpath lookup ([f02cda2](https://github.com/vanducng/vd-cli/commit/f02cda2279f40d500904e8346a15457f076012cc))
* **hooks:** tighten home relative guard ([0b35610](https://github.com/vanducng/vd-cli/commit/0b35610c1d1062f5355919f247af5817276564a7))
* **hooks:** use consistent path equality in home guard ([6b25dd4](https://github.com/vanducng/vd-cli/commit/6b25dd467a5a72d2fb1eb5f7e5486280a5333923))

## [3.2.0](https://github.com/vanducng/vd-cli/compare/v3.1.0...v3.2.0) (2026-06-17)


### Features

* **hooks:** feature-first .workbench resolver + injection (gated, type-first default) ([b47dac6](https://github.com/vanducng/vd-cli/commit/b47dac685dcd531d07b007c4f6783fc46f4f1b7e))
* **hooks:** feature-first .workbench resolver + injection (gated) ([cc48320](https://github.com/vanducng/vd-cli/commit/cc4832088c5fd8a65fa8daf66e5e53b752dc934e))


### Bug Fixes

* **hooks:** lowercase slug-only feature id + export cleanSlug ([d0a3d8e](https://github.com/vanducng/vd-cli/commit/d0a3d8e869b01e0d555fe0a709c927c12c61e1f0))

## [3.1.0](https://github.com/vanducng/vd-cli/compare/v3.0.1...v3.1.0) (2026-06-15)


### Features

* **hooks:** wire Codex notify from the manifest (codex.notify event) ([6c0276e](https://github.com/vanducng/vd-cli/commit/6c0276e75b9075075c153c273f4b0a34de05b38d))
* **hooks:** wire Codex notify from the manifest (codex.notify event) ([3b2b990](https://github.com/vanducng/vd-cli/commit/3b2b990b97d31c637145ff273436e2970b6b538f))


### Bug Fixes

* **hooks:** address review on codex.notify wiring ([9b3cffb](https://github.com/vanducng/vd-cli/commit/9b3cffb3b3a88b6034e2881a2e200aa46d5af742))

## [3.0.1](https://github.com/vanducng/vd-cli/compare/v3.0.0...v3.0.1) (2026-06-15)


### Bug Fixes

* **hooks:** harden manifest installer and hook-command builder ([15a9e4d](https://github.com/vanducng/vd-cli/commit/15a9e4d5727ad24dacfa964224ad2d21eed3a923))
* **hooks:** harden manifest installer and hook-command builder ([d2d2331](https://github.com/vanducng/vd-cli/commit/d2d2331fefea86f024727c60a6f6c9a273f37748))

## [3.0.0](https://github.com/vanducng/vd-cli/compare/v2.12.1...v3.0.0) (2026-06-15)


### ⚠ BREAKING CHANGES

* **hooks:** 'vd install hooks' now requires a hooks/hooks.toml manifest at the repo root and no longer ships built-in hooks. Run it from a repo that defines the manifest (e.g. ~/skills).

### Features

* **hooks:** source Claude hooks from a local manifest; drop embedded assets ([350e664](https://github.com/vanducng/vd-cli/commit/350e66432b182fc12cdfaaae32d7b77f4aa29747))

## [2.12.1](https://github.com/vanducng/vd-cli/compare/v2.12.0...v2.12.1) (2026-06-14)


### Bug Fixes

* **add:** strip repo from path for owner/repo/path auto-register ([#30](https://github.com/vanducng/vd-cli/issues/30)) ([effe0a4](https://github.com/vanducng/vd-cli/commit/effe0a456e052a7a98fa9450f1379df25fa767d7))

## [2.12.0](https://github.com/vanducng/vd-cli/compare/v2.11.0...v2.12.0) (2026-06-14)


### Features

* **tui:** add agent filter (Claude/Codex/Cursor) to the inventory ([bfbe7b9](https://github.com/vanducng/vd-cli/commit/bfbe7b97225896726407ebe582cd1aa68cebeaea))
* **tui:** add agent filter to the inventory ([73edde0](https://github.com/vanducng/vd-cli/commit/73edde0c42411240e6924320623463c265955b09))

## [2.11.0](https://github.com/vanducng/vd-cli/compare/v2.10.0...v2.11.0) (2026-06-14)


### Features

* **web:** multi-agent discovery + filterable inventory browser ([b0d29e1](https://github.com/vanducng/vd-cli/commit/b0d29e1d7dd11430bc2a8316f36c19505e64d83b))
* **web:** multi-agent discovery + filterable inventory browser ([9cc4373](https://github.com/vanducng/vd-cli/commit/9cc43739c6cb466e74833d04c194dafca4c67fd2))

## [2.10.0](https://github.com/vanducng/vd-cli/compare/v2.9.0...v2.10.0) (2026-06-14)


### Features

* **desktop:** add Wails desktop frontend over the inventory backend ([4732050](https://github.com/vanducng/vd-cli/commit/4732050060d01fd38370d7ed9df751a443823ec4))
* **desktop:** add Wails desktop frontend over the inventory backend ([d4e8402](https://github.com/vanducng/vd-cli/commit/d4e8402deec934eda5f55197cdaa5ceb3d576c49))

## [2.9.0](https://github.com/vanducng/vd-cli/compare/v2.8.0...v2.9.0) (2026-06-14)


### Features

* **tui:** add `vd tui` terminal frontend over the inventory backend ([4822bc0](https://github.com/vanducng/vd-cli/commit/4822bc02df9f4e67305429473d8e3fdf02b0c5ef))
* **tui:** add vd tui terminal frontend over the inventory backend ([fce88bf](https://github.com/vanducng/vd-cli/commit/fce88bf49c5ddf2b7052c18da39e3eb45850c2a8))

## [2.8.0](https://github.com/vanducng/vd-cli/compare/v2.7.0...v2.8.0) (2026-06-14)


### Features

* **install:** post-install PATH notice and Homebrew-aware upgrade hints ([7879591](https://github.com/vanducng/vd-cli/commit/7879591e9e36db77bb7dc57a0323878905416cbe))
* **web:** add `vd web` local review UI for skills, assets, and hooks ([f6ee9ea](https://github.com/vanducng/vd-cli/commit/f6ee9ea1415daab4a2e2771235de70ec6ab27fc9))
* **web:** add vd web local review UI for skills, assets, and hooks ([c193de6](https://github.com/vanducng/vd-cli/commit/c193de66d5c28b23db2196d43c77e4d48652c128))

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
