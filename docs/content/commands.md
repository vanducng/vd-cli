---
title: "vd — Command Reference"
---

One section per verb. All commands accept the global flags `--quiet` / `-q`, `--verbose` / `-v`, `--root <path>`, and `--version`.

**Repo root resolution.** Order: `--root` flag → `VD_ROOT` env var → walk up from CWD to the first `.git/` → the bootstrapped home `~/.vd/skills` (when populated by [`vd bootstrap`](#vd-bootstrap)). Set `VD_ROOT` in your shell to pin a default repo when invoking `vd` from arbitrary directories. Both `--root` and `VD_ROOT` are validated (must exist, must be a directory).

**Upstream version check.** Each command runs a background lookup against the GitHub releases API, cached for 24 hours under `$XDG_CACHE_HOME/vd/version-check.json` (or `~/.cache/vd/version-check.json`). When a newer release exists, vd prints a single line to stderr: `vd 1.0.0 (latest: 1.1.0). Upgrade: vd upgrade`. Apply it with [`vd upgrade`](#vd-upgrade) for standalone binaries or `brew update && brew upgrade vanducng/tap/vd` for Homebrew. The check is best-effort and silent on any failure. Auto-disabled when `CI` is set, when the binary is a `dev` build, and when stderr is not a terminal.

:::tip
Disable the version check globally with `VD_NO_UPDATE_CHECK=1`, or suppress it per-call with `--quiet`.
:::

---

## vd init

Create `skills.toml` at the repo root. Walks up from CWD to find `.git/`, or use `--root`.

On first run it reads `.claude-plugin/marketplace.json` (if present) and seeds the `[targets.claude.bundle]` defaults so `vd build` immediately produces byte-equal output.

**Signature:**
```
vd init [--force] [--root <path>]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--force` | Overwrite an existing `skills.toml`. |
| `--root <path>` | Use this directory as the repo root instead of walking up. |

**Examples:**
```sh
vd init                      # create skills.toml at repo root
vd init --force              # regenerate from defaults (overwrites edits)
vd init --root /tmp/myrepo   # target a specific repo
```

**Side effects:** writes `skills.toml`. Does not touch `skills.lock` or any skill directories.

**Exit codes:** `0` success, `1` file already exists without `--force`, `1` no `.git/` found.

---

## vd add

Register an upstream skill in `skills.toml`. Fetches the upstream catalog to validate the path and record the current HEAD SHA. Does **not** copy files locally — use `vd sync` for that.

If the source name (`owner` in `owner/repo/path`) is not declared in `[sources]`, it is auto-registered as a GitHub HTTPS source.

**Signature:**
```
vd add <source>/<path> [--as <name>] [--mode tracked|pinned] [--ref <ref>] [--refresh]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--as <name>` | Override the skill name in `skills.toml` (local directory name). Required when two sources share a skill name. |
| `--mode` | `tracked` (default) or `pinned`. Pinned locks to the resolved SHA immediately. |
| `--ref <ref>` | Override the branch/tag/SHA for this fetch (overrides `[sources.*].ref`). |
| `--refresh` | Bypass the cache TTL and re-fetch from upstream. |

**Examples:**
```sh
vd add browserbase/skills/browser
vd add browserbase/skills/browser --as bb-browser   # avoid name collision
vd add browserbase/skills/browser --mode pinned      # pin to current HEAD
vd add browserbase/skills/browser --ref v1.2.0       # fetch specific tag
```

**Side effects:** mutates `skills.toml` (adds `[sources.*]` if needed and `[skills.*]`). Updates the local git cache under `.vd-cache/`. Does not touch `skills.lock` or `skills/`.

**Idempotent:** re-running with the same source/path/mode is a no-op.

**Exit codes:** `0` success, `1` path not found in upstream catalog (lists available paths), `1` git not in PATH.

---

## vd list

Print tracked skills from `skills.toml` as a formatted table.

**Signature:**
```
vd list
```

**Output columns:**

| Column | Description |
|--------|-------------|
| `NAME` | Local skill name (directory under `skills/`). |
| `SOURCE` | Source key from `[sources.*]`. |
| `MODE` | `tracked`, `pinned`, or `detached`. |
| `SHA` | First 8 chars of the commit SHA from `skills.lock`. `-` if not yet synced. |
| `DRIFT` | Reserved; always `-` in the current release. |

**Examples:**
```sh
vd list
# NAME           SOURCE       MODE     SHA       DRIFT
# browser        browserbase  tracked  2a3bbb3b  -
# browser-trace  browserbase  tracked  2a3bbb3b  -
```

Prints `no skills tracked` and exits `0` on an empty manifest.

**Exit codes:** `0` always (informational command).

---

## vd sync

Fetch upstream content for all tracked and pinned skills (or a named subset) and copy them atomically into `skills/<name>/`. Updates `skills.lock`. Runs `vd build` automatically afterward unless `--no-build` is passed.

:::caution
Skills with local modifications **refuse to sync** by default (see "refuse-on-dirty" in [FAQ](/faq/)) — `--force` overwrites them and discards your local edits. Detached skills are always skipped.
:::

**Signature:**
```
vd sync [skill...] [--force] [--no-build]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--force` | Overwrite skills that have local modifications without refusing. |
| `--no-build` | Skip the automatic `vd build` after a successful sync. |

**Examples:**
```sh
vd sync                        # sync all tracked/pinned skills
vd sync browser                # sync only the "browser" skill
vd sync browser browser-trace  # sync two skills
vd sync --force                # overwrite even locally modified skills
vd sync --no-build             # sync without regenerating plugin files
```

**Side effects:** writes or updates `skills/<name>/` directories atomically (stage → rename), updates `skills.lock`, and runs `vd build` (unless `--no-build`).

**Exit codes:** `0` success, `1` any skill refused (dirty) without `--force`, `1` fetch error.

---

## vd update

Re-fetch upstream HEAD for all `tracked` skills (or a named subset) and update `skills.lock`. Pinned and detached skills are skipped.

Internally calls the same sync logic as `vd sync` but filters to mode=tracked only.

**Signature:**
```
vd update [skill...] [--force]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--force` | Overwrite skills that have local modifications without refusing. |

**Examples:**
```sh
vd update                 # bump all tracked skills to upstream HEAD
vd update browser         # bump only "browser"
```

**Side effects:** same as `vd sync` but restricted to tracked skills.

**Exit codes:** same as `vd sync`.

---

## vd diff

Show a diff between the cached upstream copy of a skill and the local `skills/<name>/` directory. Shells out to `git diff --no-index --color`.

**Signature:**
```
vd diff <skill>
```

**Examples:**
```sh
vd diff browser           # compare cache vs skills/browser/
```

**Exit codes:** `0` identical, `1` differences found (mirrors `git diff` semantics), `>1` error (git not found, cache missing, skill not in lock).

---

## vd doctor

Report drift between `skills.lock` entries and the current state of `skills/` on disk. Informational only — always exits `0`.

Reports three status values:

| Status | Meaning |
|--------|---------|
| `none` | Lock SHA matches filesystem tree hash. Clean. |
| `modified` | Filesystem tree hash differs from lock. Local edits present. |
| `missing` | Skill is in the lock but the `skills/<name>/` directory does not exist. |
| `untracked` | Directory exists in `skills/` but has no lock entry (hand-authored or detached). |

**Signature:**
```
vd doctor
```

**Examples:**
```sh
vd doctor
# SKILL           STATUS     DETAIL
# -----           ------     ------
# browser         none
# research        untracked  (hand-authored or detached — OK)
```

**Exit codes:** `0` always.

---

## vd pin

Lock a skill to a specific upstream commit SHA. Sets `mode = "pinned"` and records the SHA in `skills.toml`. Does **not** trigger a sync — run `vd sync` to apply.

**Signature:**
```
vd pin <skill> <sha>
```

The SHA must be at least 7 hex characters (short or full SHA both accepted).

**Examples:**
```sh
vd pin browser abc1234f
vd pin browser abc1234f0000000000000000000000000000000000   # full SHA OK
```

**Side effects:** mutates `skills.toml` (`mode = "pinned"`, `pin = "<sha>"`).

**Exit codes:** `0` success, `1` skill not in manifest, `1` invalid SHA format.

---

## vd detach

Stop tracking a skill from its upstream source. Sets `mode = "detached"`, clears `source`/`path`/`pin`, and removes the entry from `skills.lock`. The `skills/<name>/` directory is **left untouched** on disk.

After detaching, `vd sync` and `vd update` skip the skill entirely. You can edit the directory freely.

**Signature:**
```
vd detach <skill>
```

**Examples:**
```sh
vd detach browser         # keep files, stop syncing
```

**Side effects:** mutates `skills.toml` and `skills.lock`. Does not touch the filesystem skill directory.

**Exit codes:** `0` success (including already-detached no-op), `1` skill not in manifest.

---

## vd remove

Remove a skill from `skills.toml`, `skills.lock`, and (by default) from `skills/<name>/` on disk.

:::caution
This deletes `skills/<name>/` from disk by default. Without `--force` it refuses when the directory has local modifications (filesystem hash differs from lock); use `vd detach` first if you want to keep the edits.
:::

**Signature:**
```
vd remove <skill> [--keep-files] [--force]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--keep-files` | Remove from manifest and lock but leave `skills/<name>/` on disk. |
| `--force` | Delete even if local modifications are detected. |

**Examples:**
```sh
vd remove browser                  # remove from manifest + disk
vd remove browser --keep-files     # untrack but keep the directory
vd remove browser --force          # delete even with local edits
```

**Side effects:** mutates `skills.toml` and `skills.lock`; may delete `skills/<name>/`.

**Exit codes:** `0` success, `1` skill not in manifest, `1` local modifications detected without `--force`.

---

## vd build

Emit plugin files for all configured targets. Reads `skills.toml` and `skills.lock`.

**Targets:**
- `claude` — writes `.claude-plugin/marketplace.json` and `.claude-plugin/plugin.json`.
- `agents` — writes `.agents/skills/<name>` symlinks pointing at `skills/<name>/`.

With no arguments, both targets are built. Pass target names to build only those.

**Signature:**
```
vd build [target...]
```

**Examples:**
```sh
vd build                  # build all targets (claude + agents)
vd build claude           # regenerate marketplace.json and plugin.json only
vd build agents           # regenerate .agents/skills/ symlinks only
```

**Side effects:** writes `.claude-plugin/marketplace.json`, `.claude-plugin/plugin.json`, and `.agents/skills/<name>` symlinks. In bundle mode, output is byte-equal to the live files when manifest is seeded correctly.

**Exit codes:** `0` success, `1` unknown target name, `1` `skills.toml` not found.

---

## vd bootstrap

Clone the skills content repository (default `vanducng/skills`) into `$HOME/.vd/skills` at the latest **release tag**, so a freshly installed `vd` has skills to install from without cloning anything by hand. Afterwards `vd install` (and other commands) resolve `~/.vd/skills` as the repo root when no `--root`, `VD_ROOT`, or `.git` repo applies.

Re-run with `--update` to move an existing checkout to the latest release; pin a specific tag or branch with `--ref`.

**Signature:**
```
vd bootstrap [--repo <url>] [--ref <tag|branch>] [--update] [--dry-run]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--repo <url>` | Skills repo URL. Defaults to `vanducng/skills` or `$VD_SKILLS_REPO`. |
| `--ref <ref>` | Pin a specific tag or branch instead of the latest release. |
| `--update` | Move an existing `~/.vd/skills` to the target release (no-op without it). |
| `--dry-run` | Print what would be cloned/updated without changing files. |

**Examples:**
```sh
vd bootstrap                            # clone vanducng/skills@<latest tag> into ~/.vd/skills
vd bootstrap --update                   # move an existing checkout to the latest release
vd bootstrap --ref v0.25.0              # pin a specific release
VD_SKILLS_REPO=me/skills vd bootstrap   # use a fork
```

**Side effects:** clones or updates `$HOME/.vd/skills` (a shallow checkout at the target tag). Requires `git` ≥ 2.25 in `PATH`.

**Exit codes:** `0` success (including already-present / already-current), `1` git missing, no release tags found, or clone/fetch failure.

---

## vd install

Install local skills into an agent environment. With no agent argument, `vd install` opens a terminal picker with these choices:

1. Codex user skills — symlink to `$HOME/.agents/skills` (default recommendation)
2. Codex repo skills — symlink to `.agents/skills`
3. Codex snapshot copy — copy to `$HOME/.agents/skills`
4. Claude Code plugin — marketplace/plugin install
5. Claude Code dev symlinks — per-skill symlink into `$HOME/.claude/skills`

Pick several at once with a comma-separated list (e.g. `1,3,5`) or `all`. Passing the agent is recommended for scripts.

If no skills are found locally (no `--root`/`VD_ROOT`/`.git` repo and no `~/.vd/skills`), `vd install` offers to run [`vd bootstrap`](#vd-bootstrap) first; in non-interactive contexts it errors with that hint instead.

**Agents:**
- `codex` — installs local `skills/<name>/` directories into Codex discovery paths. Default scope is user, which writes symlinks to `$HOME/.agents/skills`. Use `--scope repo` to write `.agents/skills/<name>` in the current repo.
- `claude` — runs `vd build claude`, registers this repo as a Claude Code marketplace, and installs the configured plugin bundle.
- `claude --dev` — per-skill symlinks into `$HOME/.claude/skills` (mirrors the codex symlink flow) instead of the marketplace plugin install. Accepts skill-name arguments.
- `hooks` — install Claude Code hooks and declared Codex context hooks from a local manifest (`<root>/hooks/hooks.toml`). See [vd hooks](#vd-hooks).

**Signature:**
```
vd install [agent] [skill...]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--scope` | `codex`: `user` or `repo`; `claude`: `user`, `project`, or `local`. |
| `--dest` | Override Codex destination directory. |
| `--copy` | Copy Codex skills instead of symlinking. |
| `--force` | Replace existing Codex destination entries. |
| `--dev` | Claude only: per-skill symlink into `$HOME/.claude/skills` instead of the marketplace plugin install. |
| `--dry-run` | Print planned actions without changing files. |

**Examples:**
```sh
vd install codex                         # symlink all skills into $HOME/.agents/skills
vd install codex research plan           # install selected skills only
vd install codex --scope repo            # symlink all skills into .agents/skills
vd install codex --copy --force          # replace existing installs with copies
vd install                               # open the install target picker
vd install 1,3,5                         # picker: install multiple targets at once
vd install all                           # picker: install every target
vd install claude                        # install configured Claude Code plugin bundle
vd install claude --dev research         # symlink selected skills into ~/.claude/skills
vd install claude --dry-run              # print Claude plugin commands
vd install hooks --root ~/skills         # install hooks from a local manifest
```

**Side effects:** `codex` writes symlinks or copies under the destination skill directory. `claude` may mutate Claude Code marketplace and plugin installation state. `hooks` writes to `~/.claude/hooks/` and `~/.claude/settings.json`; manifest entries with `event = "codex.UserPromptSubmit"` also write to `~/.codex/hooks/` and `~/.codex/hooks.json` (see [vd hooks](#vd-hooks)).

**Exit codes:** `0` success, `1` invalid agent/scope, missing skill, existing destination without `--force`, or external Claude command failure.

---

## vd hooks

Install and manage hooks from a **local manifest** — `<repo-root>/hooks/hooks.toml`. Since v3.0.0 vd ships **no** built-in hooks; it installs whatever your manifest declares, so you keep your hooks in your own repo (e.g. `~/skills`) and `vd` is just the installer.

### Install

```
vd install hooks [--root <repo>] [--dry-run]
```

Reads `<root>/hooks/hooks.toml`, copies each listed file into `~/.claude/hooks/` (backing up a differing existing file first), and registers the non-`lib` Claude entries in `~/.claude/settings.json`. Manifest entries with `event = "codex.UserPromptSubmit"` are copied to `~/.codex/hooks/` with their `lib` files and registered in `~/.codex/hooks.json`. Run it from the repo holding your manifest, or point at it with `--root`:

```sh
vd install hooks --root ~/skills            # install the hooks defined in ~/skills/hooks/
vd install hooks --root ~/skills --dry-run  # preview the settings.json diff, change nothing
```

### The manifest — `hooks/hooks.toml`

One `[[hook]]` table per hook:

```toml
[[hook]]
file    = "session-init.py"                # path under hooks/ (no ".." or absolute paths)
runtime = "python3"                        # python3 | uv | node | "" (direct exec via shebang)
event   = "SessionStart"                   # a Claude event, or "statusLine"
matcher = "startup|resume|clear|compact"   # optional

[[hook]]
file    = "agent-notify.py"
runtime = "python3"
event   = "Stop"
args    = ["claude", "stop"]               # extra argv appended after the file

[[hook]]
file = "lib/config.py"
lib  = true                                # copied only, never registered (support file)
```

- **Runtimes:** `python3` (stdlib), `uv` (registered as `uv run "<file>"` — for hooks with PEP 723 inline deps; requires [uv](https://github.com/astral-sh/uv) on `PATH`, so only depend on it if a hook uses it), `node`, or empty (run the file directly via its shebang).
- **Events:** the standard Claude events (`SessionStart`, `Stop`, `Notification`, `PreToolUse`, `PostToolUse`, `SubagentStart`, `SubagentStop`, `UserPromptSubmit`, `PreCompact`, …) plus pseudo-events: `statusLine` (Claude status line), **`codex.UserPromptSubmit`** (Codex prompt context), and **`codex.notify`** (Codex `notify` program). Unknown events **and** unknown TOML fields are rejected.
- **Codex context (`codex.UserPromptSubmit`):** registers the hook in `~/.codex/hooks.json` and copies the hook plus manifest `lib` files to `~/.codex/hooks/`. Use it for the VD context injector, e.g. `file = "dev-rules-reminder.py"`, `runtime = "python3"`, `event = "codex.UserPromptSubmit"`.
- **Codex (`codex.notify`):** an entry with `event = "codex.notify"` installs the file like any other hook (into `~/.claude/hooks/`) but registers it in `~/.codex/config.toml` as the `notify` program (absolute path — Codex execs directly, no `$HOME` expansion). One file serves both agents. Any prior `notify` is replaced and reported, so you can chain it via your notifier's env (e.g. `CODEX_NOTIFY_FORWARD`). Example: `file = "agent-notify.py"`, `runtime = "python3"`, `event = "codex.notify"`, `args = ["codex"]`.
- **Registered command:** `<runtime> "$HOME/.claude/hooks/<file>" <args…>` — `$HOME` stays literal; args with shell metacharacters are quoted. `runtime = "uv"` expands to the two-token prefix `uv run`.
- The manifest is the only hook source: `[hooks].source` in `skills.toml` accepts only `local`.

### Uninstall / rollback

```
vd hooks uninstall [--dry-run]    # remove manifest files from ~/.claude/hooks and unregister settings
vd hooks rollback  [--dry-run]    # restore the most recent backup made by 'vd install hooks'
```

Only files in your manifest are removed; any other hooks in `settings.json` and `~/.codex/hooks.json` are left untouched. `settings.json` and changed hook JSON files are backed up before the first change.

### Updating a hook

Edit the source in your hooks repo (e.g. `~/skills/hooks/agent-notify.py`), then re-run `vd install hooks --root ~/skills` to sync the copy under `~/.claude/hooks/`.

### Fresh machine

```sh
git clone https://github.com/<you>/skills ~/skills
# export any secrets your hooks read — e.g. in ~/.envrc (direnv):
#   export TELEGRAM_BOT_TOKEN=...   export TELEGRAM_CHAT_ID=...
vd install hooks --root ~/skills
```

Secrets stay in the environment, never in the repo or the manifest.

---

## vd context

Print the same VD path/naming context emitted by the prompt hook. This is useful for checking `.vd.json`, workbench layout, and session plan resolution without submitting a prompt to an agent.

**Signature:**
```
vd context print [--cwd <dir>] [--session-id <id>] [--json] [--hook-path <file>]
```

**Examples:**
```sh
vd context print
vd context print --cwd /path/to/repo --session-id "$VD_SESSION_ID"
vd context print --json --cwd /path/to/repo
```

**Side effects:** none. It runs the `dev-rules-reminder` hook from `~/.codex/hooks/`, `~/.claude/hooks/`, or the local vd hooks directory and prints the resolved context. Each directory is scanned preferring the Python hook (`dev-rules-reminder.py`, run via `python3`) over the legacy Node hook (`dev-rules-reminder.cjs`, run via `node`), so a machine that upgrades vd before re-running `vd install hooks` still works.

**Exit codes:** `0` success, `1` missing hook, bad cwd, invalid hook JSON, or hook execution failure (the runtime — `python3` or `node` — not found or the hook erroring).

---

## vd cache clean

Delete the `.vd-cache/` directory at the repo root. The cache stores sparse-cloned upstream repos and is repopulated on the next `vd add` or `vd sync`.

**Signature:**
```
vd cache clean
```

**Examples:**
```sh
vd cache clean            # remove .vd-cache/
```

**Side effects:** removes `.vd-cache/` entirely. Safe to run at any time; no manifest or skill data is touched.

**Exit codes:** `0` always (no-op if cache is already empty).

---

## vd obs

Local observability over Claude Code and Codex transcripts. Read-only. See the [observability guide](/observability) for the full picture.

- `vd obs sessions [--agent claude-code|codex] [--project <p>] [--since 7d] [--limit N] [--offset N] [--json]` — list sessions across both agents.
- `vd obs show <id-or-prefix> [--turns N] [--json]` — one session turn by turn.
- `vd obs usage [--daily|--monthly] [--agent <a>] [--since 3d] [--json]` — tokens and API-equivalent cost by day or month, per model.
- `vd obs skills [--agent <a>] [--project <p>] [--since <d>] [--json]` — per-skill tool calls, error rate, corrections/aborts and tokens, attributed per invocation (window = invocation → next invocation; `(none)` = unattributed).
- `vd obs hooks [--agent <a>] [--project <p>] [--since <d>] [--json]` — hook fire counts, block rates and their share of same-turn tool errors (Claude-only).
- `vd obs health [--agent <a>] [--project <p>] [--since <d>] [--json]` — deterministic error clusters (stable signatures, prefix-key merged) with counts, guarded trends, evidence refs and resolved co-occurring skill paths; framed as an investigate signal, not a verdict.
- `vd obs sync [--full] [--agent <a>] [--since <d>]` — ingest new or changed transcripts; `--full` drops the cache and re-reads every transcript.

Costs are estimates from token counts, not a subscription bill; unpriced models render `?`. Cache at `~/.vd/obs/obs.sqlite` (override with `VD_OBS_DB`); price overrides in `~/.vd/obs/prices.json`.

## vd web

Launch a localhost-only web UI to review the assets vd manages. It serves a read-only inventory of the skills tracked in `skills.toml` (with drift status) and the assets discovered across every coding agent — **Claude Code** (`~/.claude`), **Codex** (`~/.agents`), and **Cursor** (`~/.cursor`) — each tagged by agent. The inventory is a filterable browser: stat bar, type / agent / scope chips, search, sort, and a Cards/Table toggle. Plus the registered Claude hooks and a `vd doctor` view. Nothing is written; mutating actions stay in the CLI.

Codex and Cursor discovery roots can be overridden with `VD_CODEX_HOME` / `VD_CURSOR_HOME`; missing directories are skipped.

The SPA is embedded in the binary, so no Node or network access is needed at runtime.

`web` is one frontend over a shared, transport-agnostic inventory backend (`internal/inventory`); `tui` and `desktop` (Wails) are sibling frontends that reuse the same backend.

**Signature:**
```
vd web [--port <n>] [--host <addr>] [--no-browser]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--port <n>` | Port to listen on (default `7777`). |
| `--host <addr>` | Host to bind. Loopback only; non-loopback addresses are refused (default `127.0.0.1`). |
| `--no-browser` | Do not open a browser automatically. |

**Examples:**
```sh
vd web                      # serve http://127.0.0.1:7777 and open a browser
vd web --port 8080 --no-browser
```

**API (read-only JSON):** `GET /api/inventory`, `/api/skills/{name}`, `/api/hooks`, `/api/doctor`, `/api/health`.

**Side effects:** none — reads `skills.toml`, `skills.lock`, `~/.claude`, and `~/.claude/settings.json`. Press Ctrl-C to stop.

:::note
The embedded SPA is rebuilt with `make web` (requires Node); the committed build output lives in `internal/ui/web/static/`. The plain `go build` never needs Node.
:::

**Exit codes:** `0` on clean shutdown; non-zero if the port is in use or a non-loopback host is requested.

---

## vd tui

Browse the same inventory as [`vd web`](#vd-web) in an interactive terminal UI — no browser, no server. Tabs for the managed/discovered asset inventory (with drift), the registered hooks, and a `vd doctor` view. Read-only.

`tui` and `web` are sibling frontends over the shared `internal/inventory` backend.

**Signature:**
```
vd tui
```

**Keys:** `tab` / `←` `→` switch tabs · `↑` `↓` move · `enter` open a skill's detail · `esc` back · `q` quit.

**Side effects:** none — reads `skills.toml`, `skills.lock`, `~/.claude`, and `~/.claude/settings.json`.

**Exit codes:** `0` on quit.

---

## vd upgrade

Upgrade the `vd` binary itself to the latest GitHub release. Downloads the platform archive, verifies it against the published `checksums.txt`, extracts the binary, and atomically replaces the running executable in place.

:::note
Homebrew installs are **not** self-replaced — `vd upgrade` detects them and tells you to run `brew update && brew upgrade vanducng/tap/vd` instead. This only applies when an upgrade is actually available; if you are already on the latest version it reports so and exits.
:::

**Signature:**
```
vd upgrade [--check] [--version <tag>]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--check` | Report current vs latest and whether an update is available; do not install. |
| `--version <tag>` | Install a specific release tag (e.g. `v2.3.0`) instead of the latest. |

**Examples:**
```sh
vd upgrade                 # download + self-replace with the latest release
vd upgrade --check         # just report current vs latest
vd upgrade --version v2.3.0 # install a specific release
```

**Side effects:** replaces the running binary on disk. No repo, manifest, or skill data is touched.

**Exit codes:** `0` success or already up to date, `1` Homebrew-managed install (use `brew update && brew upgrade vanducng/tap/vd`), unsupported platform, download/checksum failure, or no write permission to the binary's directory.
