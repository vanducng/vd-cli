# VD Hook Behavioral Contract

Captured from: `~/.claude/hooks/session-init.cjs`, `subagent-init.cjs`,
`lib/config.cjs`, `lib/state.cjs`, `lib/paths.cjs`
Capture date: 2026-06-09. No ck code is copied here — behavior described in own words.

---

## 1. Config Loading (`loadConfig`)

### Sources (three-layer cascade, last wins)
| Layer | Path | Notes |
|-------|------|-------|
| 1. Defaults | hardcoded `DEFAULT_CONFIG` | always applied |
| 2. Global | `os.homedir()+"/.claude/.vd.json"` | user-level prefs; falls back to `.ck.json` if absent |
| 3. Local | `path.join(gitRoot, ".vd.json")` | project-local; falls back to `.ck.json` if absent |

Both layers use `path.join(os.homedir(), ".claude", ".vd.json")` for global and
`path.join(gitRoot, ".vd.json")` for project-local. Legacy `.ck.json` read fallback
keeps existing setups working until migrated. Write always goes to `.vd.json`.

### Merge semantics (`deepMerge`)
- Arrays: replaced entirely (not concatenated).
- Plain objects: merged recursively.
- Empty object `{}` in source: treated as "inherit from parent" (skipped).
- Primitives: source wins.

### Key `.vd.json` fields consumed by session-init / subagent-init

| Field path | Type | Default | Purpose |
|---|---|---|---|
| `plan.namingFormat` | string | `"{date}-{issue}-{slug}"` | template for `VD_NAME_PATTERN` |
| `plan.dateFormat` | string | `"YYMMDD-HHmm"` | format tokens: `YYYY YY MM DD HH mm ss` |
| `plan.issuePrefix` | string\|null | `null` | prepended to issue number extracted from branch |
| `plan.reportsDir` | string | `"reports"` | subdirectory under plan dir (or plans/) |
| `plan.resolution.order` | string[] | `["session","branch"]` | plan lookup order |
| `plan.resolution.branchPattern` | string | `"(?:feat\|fix\|chore\|refactor\|docs)/(?:[^/]+/)?(.+)"` | regex to extract slug from branch |
| `paths.docs` | string | `"docs"` | base-relative docs dir |
| `paths.plans` | string | `"plans"` | base-relative plans dir |
| `locale.thinkingLanguage` | string\|null | `null` | injected into subagent `## Language` block |
| `locale.responseLanguage` | string\|null | `null` | injected into subagent `## Language` |
| `codingLevel` | int | `-1` | -1 = disabled; 0-5 = load output-style guidelines |
| `hooks.<name>` | bool | `true` | false disables hook early-exit |
| `assertions` | string[] | `[]` | printed to console at SessionStart |

---

## 2. How `session-init.cjs` Is Invoked

**Hook event:** `SessionStart` (fires on startup, resume, clear, compact).

**stdin:** JSON object, shape:
```json
{
  "session_id": "<uuid-string-or-null>",
  "source":     "startup" | "resume" | "compact" | "clear" | "unknown",
  "hook_event_name": "SessionStart",
  "transcript_path": "<path-or-absent>"
}
```

**Env vars read:**
| Var | Used for |
|-----|----------|
| `CLAUDE_ENV_FILE` | path of the shell env file to append `export KEY="VAL"` lines to |
| `CLAUDE_SESSION_ID` | fallback session id if `stdin.session_id` absent (not used in code; stdin is primary) |

**Process CWD:** the Claude project directory at session start — `baseDir = process.cwd()`.

**Output written:**
- Appends `export KEY="VAL"\n` lines to `CLAUDE_ENV_FILE` (one line per var, shell-escaped).
- Prints a single-line context summary to stdout.
- May print session-state recovery block, team-detection lines, coding-level guidelines.

**Escape rule (`escapeShellValue`):** `\` → `\\`, `"` → `\"`, `$` → `\$`, `` ` `` → `` \` ``.

---

## 3. The 8 Load-Bearing Env Vars — Derivation Rules

### 3.1 `VD_SESSION_ID`
**Source:** `stdin.session_id` (raw UUID from Claude Code).  
**Formula:** written verbatim; empty string `""` if null.  
**Volatile:** yes — new UUID each session.  
**Mask token:** `{{SESSION_ID}}`

### 3.2 `VD_GIT_ROOT`
**Source:** `getGitRoot()` → `git rev-parse --show-toplevel` run in `process.cwd()`.  
**Formula:** absolute path string, or empty string `""` if not in a git repo.  
**Anchoring:** git root of the CWD at SessionStart; may differ from CWD in subdirectory workflows.  
**Volatile:** no (stable per project); masked because it's a machine-absolute path.

### 3.3 `VD_PLANS_PATH`
**Source:** `config.paths.plans` (default `"plans"`).  
**Formula:** `path.join(baseDir, config.paths.plans)` where `baseDir = process.cwd()`.  
**Anchoring:** CWD-based (not git-root), per Issue #327.  
**Example (default):** `<CWD>/plans`

### 3.4 `VD_DOCS_PATH`
**Source:** `config.paths.docs` (default `"docs"`).  
**Formula:** `path.join(baseDir, config.paths.docs)`.  
**Anchoring:** CWD-based.  
**Example (default):** `<CWD>/docs`

### 3.5 `VD_REPORTS_PATH`
**Source:** `getReportsPath(resolved.path, resolved.resolvedBy, config.plan, config.paths)`.  
**Formula — two cases:**
- If `resolved.resolvedBy === "session"` (plan explicitly active):
  `path.join(baseDir, <activePlanPath>, config.plan.reportsDir)`
  (if `activePlanPath` is already absolute, `path.join` uses it as-is)
- Otherwise (no plan, or branch-suggested):
  `path.join(baseDir, config.paths.plans, config.plan.reportsDir)`
  i.e. `<CWD>/plans/reports` with defaults.

**Trailing slash:** written with a trailing `/` when no `baseDir` is passed (relative mode).
When `baseDir` IS passed (session-init path), no trailing slash.  
**Note:** session-init writes `path.join(baseDir, reportsPath)` where `reportsPath` is already relative — result has no trailing slash. Golden shows trailing `/` because `getReportsPath` appends it in the relative branch; session-init then does `path.join(baseDir, reportsPath)` which strips it. **Actual written value has trailing `/`** (verified from golden).

### 3.6 `VD_NAME_PATTERN`
**Source:** `resolveNamingPattern(config.plan, gitBranch)`.  
**Formula:**
1. Format date: expand `config.plan.dateFormat` tokens (`YYMMDD-HHmm` → e.g. `260608-1430`).
2. Extract issue from branch: match patterns like `/issue-123/`, `/#42/`, `/fix/gh-7/` → digit group.
3. If issue found AND `issuePrefix` set: `fullIssue = issuePrefix + issueId` (e.g. `GH-88`).
4. Substitute `{date}` → formatted date in `namingFormat`.
5. If `fullIssue` exists: substitute `{issue}` → `fullIssue`. Else: remove `{issue}` token and clean up extra hyphens.
6. `{slug}` is kept as a literal placeholder for agents to substitute.
7. Clean: strip leading/trailing hyphens, collapse `--` → `-`.

**Example (defaults, branch=main, no issue):** `260608-1430-{slug}`  
**Example (issuePrefix=GH-, branch=feat/gh-88-my-feat):** `260608-1430-GH-88-{slug}`  
**Volatile:** date+time portion changes every minute. `{slug}` is a stable placeholder.  
**Mask token:** `{{DATE}}-{{TIME}}-{slug}` (or `{{DATE}}-{{TIME}}-GH-{{ISSUE}}-{slug}` when issue present)

### 3.7 `VD_ACTIVE_PLAN`
**Source:** `resolvePlanPath(sessionId, config)` where `resolvedBy === "session"`.  
**Formula:** if the temp session file `$TMPDIR/vd-session-<sessionId>.json` contains
`activePlan` (non-null), that value is used. Absolute paths used as-is; relative paths
resolved via `state.sessionOrigin`.  
**Written as:** the absolute plan directory path, or empty string `""` if not session-resolved.

### 3.8 `VD_SUGGESTED_PLAN`
**Source:** `resolvePlanPath(sessionId, config)` where `resolvedBy === "branch"`.  
**Formula:** if `resolvePlanPath` returns `resolvedBy === "branch"`, the matched plan
directory path is written here; otherwise empty string `""`.  
**Branch match rule:** extract slug from `gitBranch` using `branchPattern` regex (group 1);
scan `config.paths.plans` directory for a subdirectory whose name `.includes(slug)`; if
found, use last match. No plan directory traversal if the plans dir doesn't exist.  
**Note:** `VD_ACTIVE_PLAN` and `VD_SUGGESTED_PLAN` are mutually exclusive — one is always `""`.

---

## 4. Full `~/.claude/session.json` — Actually `$TMPDIR/vd-session-<id>.json`

The file is NOT at `~/.claude/session.json`. It lives at:
```
os.tmpdir() + "/vd-session-" + sessionId + ".json"
```
(function `getSessionTempPath` in `config.cjs`)

### Schema (all known keys)

```json
{
  "sessionOrigin":      "<string: CWD at SessionStart>",
  "activePlan":         "<string|null: absolute plan path set by set-active-plan>",
  "suggestedPlan":      "<string|null: branch-matched plan path — NOTE: session-init writes null here, not the branch match>",
  "timestamp":          "<number: Date.now() at last write>",
  "source":             "<string: SessionStart source field>",
  "statusline": {
    "sessionStart":     "<ISO string>",
    "updatedAt":        "<ISO string>",
    "warmed":           "<boolean>",
    "agents":           "<array of agent objects>",
    "todos":            "<array of todo objects>"
  },
  "devRulesReminder": {
    "scopes": {
      "<cwd-path>": {
        "lastInjectedAt": "<ISO string>",
        "pendingAt":      "<ISO string (transient)>"
      }
    }
  },
  "lastTranscriptPath": "<string: path to session .jsonl transcript>"
}
```

**Note:** `session-init` writes `activePlan` = branch-resolved path when `resolvedBy === "session"`,
and `suggestedPlan` would be set for branch — but the actual `updateSessionState` call in
`session-init.cjs` sets `activePlan` only when `resolvedBy === "session"`, `null` otherwise.
The key `suggestedPlan` written in the session file is always `null` from `session-init`'s
update (session-init doesn't write the branch suggestion into the file).

### `resolvePlanPath` rule

```
resolvePlanPath(sessionId, config):
  for method in config.plan.resolution.order  // default: ["session", "branch"]
    if method == "session":
      state = readSessionState(sessionId)      // reads $TMPDIR/vd-session-<id>.json
      if state.activePlan:
        path = state.activePlan
        if !isAbsolute(path) && state.sessionOrigin:
          path = join(state.sessionOrigin, path)
        return { path, resolvedBy: "session" }
    if method == "branch":
      branch = git branch --show-current
      slug = extractSlugFromBranch(branch, branchPattern)
      if slug && plansDir exists:
        entries = list plansDir dirs containing slug
        if entries.length > 0:
          return { path: join(plansDir, last entry), resolvedBy: "branch" }
  return { path: null, resolvedBy: null }
```

**`extractTaskListId`:** returns `path.basename(resolved.path)` only when
`resolvedBy === "session"`, else `null`. This is written to `CLAUDE_CODE_TASK_LIST_ID`
(only if non-null).

### `updateSessionState` write behavior

1. Acquire file lock: create `<tempPath>.lock` with `O_EXCL` (exclusive create).
   - Retry every 10ms up to 500ms timeout; remove stale locks older than 5s.
2. Read current state (or `{}`).
3. Apply updater (either merge-object or transform function).
4. Atomic write: write to `<tempPath>.<random>.json`, then `rename()` to `<tempPath>`.
5. Release lock (close fd + unlink lock file).

---

## 5. Subagent Injection Block Structure (`subagent-init.cjs`)

**Hook event:** `SubagentStart` (fires when a Task tool call spawns a subagent).

**stdin:** JSON object:
```json
{
  "session_id":       "<string|null>",
  "agent_id":         "<string>",
  "agent_type":       "<string>",
  "cwd":              "<string: subagent's working directory>",
  "hook_event_name":  "SubagentStart"
}
```

**Output format:** JSON to stdout:
```json
{
  "hookSpecificOutput": {
    "hookEventName": "SubagentStart",
    "additionalContext": "<multiline string>"
  }
}
```

### Injection block — exact section order and template

```
## Subagent: <agent_type>
ID: <agent_id> | CWD: <effectiveCwd>

## Context
- Plan: <activePlan>          (if session-resolved)
  OR
- Plan: none | Suggested: <suggestedPlan>   (if branch-resolved)
  OR
- Plan: none                  (if no plan)
[- Task List: <taskListId> (shared with session)   ← only if session-resolved and taskListId non-null]
- Reports: <reportsPath>      (absolute, no trailing slash)
- Paths: <plansPath>/ | <docsPath>/

[## Language                  ← OMITTED if neither thinkingLanguage nor responseLanguage set]
[- Thinking: Use <lang> for reasoning (logic, precision).   ← if effectiveThinking set and != responseLanguage]
[- Response: Respond in <responseLanguage> (natural, fluent).   ← if responseLanguage set]
[]

## Rules
- Reports → <reportsPath>
- YAGNI / KISS / DRY
- Concise, list unresolved Qs at end
[- Python scripts in .claude/skills/: Use `<skillsVenv>`   ← if venv found]
[- Never use global pip install                              ← if venv found]

## Naming
- Report: <reportsPath>/<agentType>-<namePattern>.md
- Plan dir: <plansPath>/<namePattern>/

[## Plan CLI (deterministic updates)     ← ONLY for plan-aware agents]
[`ck plan check <id>` = completed | `ck plan check <id> --start` = in-progress | `ck plan uncheck <id>` = revert]
[Fallback: if `ck` unavailable, edit plan.md Status column directly.]

[## Trust Verification         ← ONLY if config.trust.enabled && config.trust.passphrase]
[Passphrase: "<passphrase>"]

[## Agent Instructions         ← ONLY if config.subagent.agents.<agentType>.contextPrefix set]
[<contextPrefix text>]
```

**Plan-aware agents** (get `## Plan CLI` section):
`planner`, `project-manager`, `code-simplifier`, `brainstormer`, `code-reviewer`, `fullstack-developer`

**Language section logic:**
- `effectiveThinking = thinkingLanguage || (responseLanguage ? "en" : "")`
- `## Language` block emitted iff `(effectiveThinking && effectiveThinking !== responseLanguage) || responseLanguage`
- Thinking line: iff `hasThinking = effectiveThinking && effectiveThinking !== responseLanguage`
- Response line: iff `responseLanguage` set

**Path anchoring:** uses `effectiveCwd = payload.cwd?.trim() || process.cwd()` as `baseDir`.
Paths are absolute, no trailing slash (except `<plansPath>/` and `<docsPath>/` in the Paths line).

**`## Rules` venv lines:** `resolveSkillsVenv()` checks (in order):
1. `<effectiveCwd>/.claude/skills/.venv/bin/python3` → relative path `<configDirName>/skills/.venv/bin/python3`
2. `~/.claude/skills/.venv/bin/python3` → literal `~/.claude/skills/.venv/bin/python3`
Returns null if neither exists; venv lines omitted when null.

**`## Naming` paths:**
- Report: `path.join(reportsPath, agentType + "-" + namePattern + ".md")`
- Plan dir: `path.join(plansPath, namePattern) + "/"`

---

## 6. `session-state.cjs` Coexistence (auxiliary hook)

`session-state.cjs` also fires on `SessionStart`/`Stop`/`SubagentStop`. It reads/writes
the same `$TMPDIR/vd-session-<id>.json` file via `readSessionState` / `updateSessionState`
from `config.cjs`.

**Keys it reads from the session file:**
- `statusline` (entire snapshot object)
- `lastTranscriptPath`
- `devRulesReminder` (via `context-builder.cjs`)

**Keys it writes:**
- `statusline` (updated activity snapshot)
- `lastTranscriptPath`

**Coexistence requirement:** our clean-room writer must write a **superset-compatible**
session file. All existing keys must be preserved on update (the `updateSessionState`
spread-merge handles this). Never drop `devRulesReminder` or `statusline` keys on write.

---

## 7. Full `VD_*` Variable Inventory

29 vars emitted by `session-init`. Organized as: 8 core (load-bearing) + 21 non-core.

### Core (8) — P2 must reproduce exactly

| Var | Keep | Notes |
|-----|------|-------|
| `VD_SESSION_ID` | KEEP | per-session UUID |
| `VD_GIT_ROOT` | KEEP | absolute git root path |
| `VD_PLANS_PATH` | KEEP | CWD-anchored absolute plans dir |
| `VD_DOCS_PATH` | KEEP | CWD-anchored absolute docs dir |
| `VD_REPORTS_PATH` | KEEP | absolute; plan-specific if session-active |
| `VD_NAME_PATTERN` | KEEP | date+issue resolved; `{slug}` placeholder kept |
| `VD_ACTIVE_PLAN` | KEEP | absolute path or empty string |
| `VD_SUGGESTED_PLAN` | KEEP | absolute path or empty string |

### Non-core (21) — P2 recommendation

| Var | Recommendation | Notes |
|-----|---------------|-------|
| `VD_PLAN_NAMING_FORMAT` | KEEP | raw config value; skills reference it |
| `VD_PLAN_DATE_FORMAT` | KEEP | raw config value |
| `VD_PLAN_ISSUE_PREFIX` | KEEP | raw config value |
| `VD_PLAN_REPORTS_DIR` | KEEP | raw config value |
| `VD_PROJECT_ROOT` | KEEP | same as baseDir/CWD; referenced by some skills |
| `VD_GIT_BRANCH` | KEEP | branch name; referenced by `session-state.cjs` |
| `VD_PROJECT_TYPE` | KEEP | `single-repo`/`monorepo`/`library` |
| `VD_PACKAGE_MANAGER` | KEEP | `npm`/`pnpm`/`yarn`/`bun` or empty |
| `VD_FRAMEWORK` | KEEP | `next`/`react`/etc. or empty |
| `VD_NODE_VERSION` | KEEP | `process.version` |
| `VD_OS_PLATFORM` | KEEP | `process.platform` |
| `VD_USER` | KEEP | `USERNAME`/`USER`/`LOGNAME`/`os.userInfo().username` |
| `VD_LOCALE` | KEEP | `process.env.LANG` |
| `VD_TIMEZONE` | KEEP | `Intl.DateTimeFormat().resolvedOptions().timeZone` |
| `VD_CLAUDE_SETTINGS_DIR` | KEEP | `path.resolve(__dirname, "..")` = `~/.claude` |
| `VD_VALIDATION_MODE` | KEEP | `prompt`/`auto`/`off` |
| `VD_VALIDATION_MIN_QUESTIONS` | KEEP | integer as string |
| `VD_VALIDATION_MAX_QUESTIONS` | KEEP | integer as string |
| `VD_VALIDATION_FOCUS_AREAS` | KEEP | comma-separated |
| `VD_CODING_LEVEL` | KEEP | integer as string; `-1` = disabled |
| `VD_CODING_LEVEL_STYLE` | KEEP | style name string |
| `VD_THINKING_LANGUAGE` | KEEP (conditional) | written only if `config.locale.thinkingLanguage` truthy |
| `VD_RESPONSE_LANGUAGE` | KEEP (conditional) | written only if `config.locale.responseLanguage` truthy |
| `CLAUDE_CODE_TASK_LIST_ID` | KEEP (conditional) | written only if session-active plan; value = `path.basename(activePlanPath)` |
| `VD_AGENT_TEAM` | KEEP (conditional) | written only if team config found in `~/.claude/teams/` |
| `VD_AGENT_TEAM_MEMBERS` | KEEP (conditional) | written only if team config found |

**Total: 29 unconditional + up to 6 conditional.**

---

## 8. Volatile Fields — Masking Map

| Token | What it masks | Position in golden |
|-------|--------------|-------------------|
| `{{SESSION_ID}}` | `VD_SESSION_ID` value | line 1 |
| `{{DATE}}-{{TIME}}` | 6-digit date + 4-digit time in `VD_NAME_PATTERN` | line 6 |
| `{{GIT_ROOT}}` | absolute repo path in `VD_GIT_ROOT`, `VD_PROJECT_ROOT`, `CWD:` line | multiple |
| `{{REPORTS_ABS}}` | `VD_REPORTS_PATH`, `Reports:` lines | multiple |
| `{{PLANS_ABS}}` | `VD_PLANS_PATH`, `Paths:` and `## Naming` lines | multiple |
| `{{DOCS_ABS}}` | `VD_DOCS_PATH`, `Paths:` line | subagent context |
| `{{HOME}}` | `VD_CLAUDE_SETTINGS_DIR` value | line 23 |
| `{{CUSTOM_REPORTS_ABS}}` | custom-fixture `VD_REPORTS_PATH` (`plans/my-reports`) | custom env line 10 |

**Machine-specific but structurally stable (not masked, noted):**
- `VD_USER` — system username
- `VD_LOCALE` — system locale
- `VD_TIMEZONE` — system timezone
- `VD_NODE_VERSION` — installed Node version

---

## 9. Config File Migration: `.ck.json` → `.vd.json`

Our clean-room implementation uses `.vd.json` as the config file name. Both the
Node hooks (`config.cjs`) and the Go CLI (`claudeconfig/config.go`) implement a
**legacy read fallback**: if `.vd.json` is absent, they silently read `.ck.json`
instead. This keeps existing setups working until users rename their config.

**Write always targets `.vd.json`** — the fallback is read-only.

Legacy fallback locations:
- `internal/hooks/assets/lib/config.cjs` — `readJsonWithLegacyFallback()`
- `internal/claudeconfig/config.go` — `ReadCKConfig()` two-phase read

---

## 10. Reproducibility Notes

- Run `node parity.mjs` from the hooks directory to run golden regression tests.
- Harness uses fake `HOME` with `.vd.json` to isolate config; does not mutate
  `~/.claude/.vd.json`.
- `VD_USER`, `VD_LOCALE`, `VD_TIMEZONE` reflect the machine running capture — expected to
  differ across machines; parity tests mask or ignore these three lines.
- `VD_NODE_VERSION` reflects installed Node — mask if testing across Node versions.
