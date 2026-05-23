# Research: vd positioning — unit + category nouns

_Date: 2026-05-23 · Mode: --deep · Queries: 7_

## TL;DR

- **Recommendation:** Keep **"skill"** (unit) + **"package manager"** (category). Inject **"vendoring"** as a differentiating adjective. Final shape: *"vendoring package manager for coding-agent skills."*
- **Runner-up:** *"skill toolchain"* — wins only if avoiding the crowded "package manager" category outweighs ecosystem recognition. It doesn't, today.
- **Avoid:** Coining a new unit noun (kit / bundle / recipe / chart / artifact). Anthropic, OpenAI, and every major 2026 competitor converged on "skill"; the SKILL.md filename hard-codes it at the filesystem layer. Going off-vocabulary is pure SEO and cognitive tax.

## The Question

Two paired decisions about how vd presents itself in a 2-second README/description scan:

1. **Unit noun** — what does vd manage? (skill, package, artifact, toolset, capability, extension, plugin, recipe, chart, module, bundle, kit)
2. **Category noun** — what category does vd belong to? (package manager, vendoring tool, dependency manager, registry, orchestrator, dispatcher, hub, marketplace, toolchain)

The decision is irreversible-ish — once it lands in a GitHub description, brew formula description, and homepage tagline, it shapes how the project is found and understood.

## Evaluation Criteria

- **Ecosystem fit** — does the term match what Anthropic, OpenAI, and the broader coding-agent ecosystem already call this thing? Going off-vocabulary loses recognition.
- **Category recognition** — when a developer scans `gh search "agent skills"`, do they immediately know what vd is?
- **Differentiation** — does the framing surface vd's unique angle (vendoring + SHA lock + multi-target dispatch) vs the registry/runtime crowd (Skills.sh, AgentPkg, APM)?
- **Collision / SEO** — naming overlap with established tools or generic words that dilute search.
- **Future-proofness** — if vd grows beyond skills (Cursor rules, Cline rules, MCP configs), does the term constrain?
- **Brevity / scanability** — fits in a 70-char GitHub description without losing meaning.

## Options Considered

Unit nouns (12):
- **skill** — Anthropic/Codex term-of-art; SKILL.md is the filename
- **package** — generic (npm, brew, dbt); used by competitors as parent category
- **artifact** — build-output flavored; doesn't fit source files
- **toolset** — implies a collection-per-unit; mismatch
- **capability** — accurate semantically but verbose
- **extension** — VS Code / Chrome convention; not used in agent-skills space
- **plugin** — Anthropic uses this for the *bundle* layer (plugin = skills + commands + hooks)
- **recipe** — Chef / OpenWrt; not adopted here
- **chart** — Helm-coined; works because Helm owns it
- **module** — Terraform / Go; implies linked code
- **bundle** — collides with Anthropic's "plugin" already being a bundle
- **kit** — vague brand-driven word

Category nouns (10):
- **package manager** — npm, brew, dbt, Helm; the dominant 2026 framing for the agent-skills space too
- **vendoring tool** — accurate to vd's distinctive angle
- **dependency manager** — synonymous with package manager but rarer in CLI tool naming
- **registry** — wrong for vd (vd is a client, not a host)
- **orchestrator** — cloud / k8s flavored
- **dispatcher** — only describes `vd build`, not the vendoring half
- **hub** — wrong (vd is the client)
- **marketplace** — wrong (vd doesn't sell or host)
- **toolchain** — accurate but unconventional in this space
- **kit manager** — too vague

## Ecosystem Landscape (May 2026)

The 2026 agent-skills space is more crowded than expected. Direct positioning competitors:

| Tool | Unit noun | Category framing | Hosting model |
|---|---|---|---|
| **Anthropic Claude Code** | skill | "Extend Claude with skills" + plugins | SKILL.md, plugin marketplace |
| **OpenAI Codex** | skill | "Agent Skills" (open standard with Anthropic) | SKILL.md in `.agents/skills/` + plugins |
| **Microsoft APM** ([github.com/microsoft/apm](https://github.com/microsoft/apm)) | skill, prompt, instruction, plugin, MCP | **Agent Package Manager** | apm.yml manifest, install per platform |
| **Skills.sh (Vercel)** | skill | registry / marketplace, **18 agents supported** | hosted registry, tracks install counts |
| **AgentPkg** ([agentpkg.com](https://www.agentpkg.com/)) | skill | **Agent Package Registry** | registry-hosted |
| **Skilldex** ([arxiv 2604.16911](https://arxiv.org/abs/2604.16911)) | skill, **skillset** | "package manager and registry for agent skill packages" | hierarchical scope-based distribution |
| **JFrog** | skill (positioning) | "Agent Skills are the New Packages of AI" | enterprise registry |
| **SkillsMP** | skill | community discovery (351k SKILL.md files indexed) | GitHub crawler |
| **dbt** | package | Package manager (`dbt deps`) | dbt Hub registry + git + local |
| **Helm** | **chart** | "Package manager for K8s" | OCI chart registries |
| **Cline** | rule | (no package manager) | `.clinerules/` per-repo |
| **Cursor** | rule | (no package manager) | cursor rules per-repo |

Five observations from this landscape:

1. **"Skill" is the convergent unit term**, full stop. Anthropic + OpenAI joint open standard. APM, Skilldex, AgentPkg, Skills.sh, JFrog, SkillsMP — all use "skill." Picking anything else fights ecosystem gravity.

2. **"Package manager" is the prevailing category framing** for this space in 2026. APM literally embeds it in the product name; Skilldex's title uses the phrase; JFrog blogs "Agent Skills are the New Packages." The 2026 industry narrative is "skills are the new npm packages." Going off-category loses SEO.

3. **Most competitors are registries**, not vendoring tools. APM, Skills.sh, AgentPkg, Skilldex all involve some hosted catalog + install flow. vd is genuinely different: it vendors into your repo as plain files, hashes them, locks them, and emits per-agent layouts. dbt deps is the closest structural analog (vendor + lockfile + multi-source), not the registries.

4. **"Plugin" is already taken** as the *bundle* layer above skills. A plugin contains skills + commands + hooks + MCP configs. So vd can't use "plugin" for its unit without colliding with Anthropic's hierarchy.

5. **Cursor and Cline use "rule"**, not "skill" — they have no skills convention. Cline has `.clinerules/`, Cursor has `.cursorrules` / project rules. If vd ever grows to support those agents, the unit noun would need to be umbrella-able (e.g., "extension" or "agent context") — but for *today*, every agent vd supports uses "skill."

## Comparison: Unit Noun

| Criterion | skill | package | extension | recipe | bundle / kit |
|---|---|---|---|---|---|
| Ecosystem term-of-art | **✓ Anthropic/OpenAI** | ✗ overloaded | ✗ wrong space | ✗ unused here | ✗ collides with plugin |
| Matches SKILL.md filename | **✓ literal match** | ✗ | ✗ | ✗ | ✗ |
| Used by competitors | **✓ all of them** | ✗ they call vd-likes "package managers" but the unit is "skill" | ✗ | ✗ | ✗ |
| Search / SEO | medium (slight overload w/ HR) | high but generic | low | low | very low |
| Future-proof if vd adds rules | weak (skill ≠ rule) | medium (could umbrella) | medium | weak | weak |
| Cognitive load | **lowest** — ecosystem readers know it | medium (too generic) | medium | high | high |
| Dealbreaker | none today | wrong layer (unit ≠ category) | nobody calls them this | not adopted | collides with plugin |

**Verdict: "skill."** Every signal converges. The filesystem hard-codes it (SKILL.md), the two agents vd supports both ship it as their term, and every direct competitor uses it. The only real cost is overload risk with the generic English word "skill," which the qualifier "coding-agent" (e.g., "coding-agent skill") resolves cleanly.

## Comparison: Category Noun

| Criterion | package manager | vendoring tool | dependency manager | registry | toolchain |
|---|---|---|---|---|---|
| 2026 ecosystem framing | **✓ dominant** | partial — accurate but uncommon phrasing | ✓ adjacent / synonymous | ✗ wrong (vd is client) | ✗ unconventional in this space |
| Differentiates from registries | ✗ — APM also says this | **✓ strong** — vd doesn't host | ✓ — implies lockfile, no host | n/a (it IS the registries) | ✓ |
| Search recognition | **highest** — "agent skills package manager" is the searched phrase | low — niche term outside Go/Ruby vendoring | medium | medium | low |
| Implies lock + reproducibility | medium (npm-flavored) | **strongest** — vendoring → vendored | ✓ | weak | medium |
| Implies multi-target dispatch | weak | weak | weak | weak | ✓ medium |
| Conflict with Helm/dbt analogy | none — both call themselves PMs | none | none | n/a | rarely used |
| Dealbreaker | doesn't differentiate from APM/Skills.sh on its own | too niche for cold scan | none | inaccurate for vd | doesn't match the category 2026 actually uses |

**Verdict:** "package manager" is the category 2026 settled on. To differentiate vd from registry-flavored package managers (APM, Skills.sh, AgentPkg), pair it with the **"vendoring"** adjective. That single word does the heavy lifting: it signals "no server, no runtime registry — skills live in your repo as files with a lock."

## Per-Option Deep Dive

### "skill" (unit) — RECOMMENDED

- **Strengths:** Ecosystem-convergent. Hard-coded by SKILL.md. Used by every direct competitor. Lowest cognitive load for the target audience.
- **Weaknesses:** Generic English word — search overlap with HR/gaming. Risks looking *un*-distinctive when paired with a generic category noun.
- **Dealbreakers:** None today. Possible future risk: if vd grows to manage Cursor `.cursorrules`, Cline `.clinerules/`, or MCP server configs — "skill" doesn't umbrella those. Mitigation: keep "skill" for the primary unit, document adjacent units (rules, MCP configs) as separate concepts in the roadmap rather than forcing them under one umbrella term.

### "package" (unit) — REJECT

- **Strengths:** Generic, instantly understood as "a unit of installable software."
- **Weaknesses:** Wrong layer. In this ecosystem, "package" is the CATEGORY (package manager) and the contained thing is "skill." Saying vd manages "packages" creates a recursive confusion ("is vd a package manager for packages?"). dbt called its units "packages" because dbt predates the agent-skills convention by years; vd doesn't have that pass.
- **Dealbreaker:** Wrong layer. Calling skills "packages" puts vd at odds with Anthropic's own docs and every Codex doc that mentions `.agents/skills/`.

### "package manager" (category) — RECOMMENDED

- **Strengths:** The 2026 category as recognized by APM, Skilldex, dbt, Helm, the JFrog narrative. Searchable. Familiar. No translation needed when explaining vd to a new dev.
- **Weaknesses:** Doesn't on its own differentiate vd from registry-style competitors. Solved by adding "vendoring."
- **Dealbreakers:** None.

### "vendoring tool" (category) — RUNNER-UP

- **Strengths:** Most accurate single phrase. Captures vd's distinctive value (no host, SHA-locked files in repo). Same shape as Go vendoring, Ruby Bundler, dbt deps.
- **Weaknesses:** Niche term. Most developers in the agent-skills space won't search "vendoring tool" — they'll search "agent skills package manager" and "skill manager for Claude Code." Lower discoverability.
- **Dealbreakers:** Reduced search recognition. Use as an *adjective on* "package manager," not as a replacement.

### "registry" (category) — REJECT

- **Strengths:** Established term in dev tooling.
- **Weaknesses:** Inaccurate. vd is a *client* of registries (it pulls from GitHub, plugin marketplaces, raw URLs). Calling vd a registry mis-sets expectations and conflicts with what AgentPkg / Skills.sh actually are.
- **Dealbreaker:** Wrong category. vd is consumer-side, registries are producer-side.

### "toolchain" (category) — REJECT for category, INTERESTING for tagline

- **Strengths:** Implies multi-step pipeline (fetch → lock → emit), which vd actually has.
- **Weaknesses:** Not the category dev tools use in 2026 for this space.
- **Dealbreaker:** Outside the recognized category vocabulary.

## Failure Modes

| Failure | Trigger | Symptom | Mitigation |
|---|---|---|---|
| Misidentified as a registry | Description leans on "manager" without "vendoring" | Users expect a hosted catalog; complain when they have to point at upstream URLs themselves | Lead with "vendoring" so expectation is set in the first sentence |
| Drowned in SEO by APM / Skills.sh | Description copies the generic "Agent Package Manager" phrasing | vd gets buried under the funded competitors | Use distinctive "vendoring" + Claude/Codex name-drop; lean on the *client-side* nature |
| Limits to skills only | Unit noun "skill" baked into copy, then Cursor / Cline support added | README/CHANGELOG suddenly contradict — "vd manages skills (and also rules?)" | Treat "skill" as the *primary* unit; note in roadmap that adjacent units (rules, MCP) may join as siblings, not under the "skill" umbrella |
| Plugin-vs-skill confusion | README conflates the two | Users send Claude `plugin.json` patches when they meant SKILL.md | The Supported agents table already covers this — keep it |

## Migration Paths

- **From current copy ("Package manager for coding agent skills") → recommended ("Vendoring package manager for coding-agent skills"):** zero-risk. One-word insertion. Affects: GitHub description, README intro, goreleaser brew description.
- **From recommended → runner-up ("Skill vendoring tool"):** higher cost. Drops the recognized category framing. Only worth it if vd ever wants to differentiate from APM/Skills.sh hard enough to sacrifice search recognition. Don't do this preemptively.
- **From "skill" → any other unit noun:** very high cost. Touches docs, CHANGELOG, every internal doc reference. Not recommended unless the ecosystem itself shifts terminology (which would happen ecosystem-wide, not project-by-project).

## Decision Reversibility

Adding "vendoring" to the current copy is fully reversible — it's a one-word edit. Locking in a non-"skill" unit noun is structurally hard to reverse once it propagates through code paths (`skills.toml`, `skills.lock`, `skills/` directory layout, the SKILL.md filename itself). For this reason, the *unit* decision matters more than the *category* adjective decision, and the unit decision is settled by the ecosystem and the filesystem.

## Operational War Stories (industry context)

- **JFrog** ([blog post Jan 2026](https://jfrog.com/blog/agent-skills-new-ai-packages/)) frames the agent-skills space as the next-npm and explicitly notes the security/distribution gap that vendoring + lockfile addresses. Their angle: enterprises will treat skills like dependencies *because* skills carry executable behavior. vd's vendoring + SHA-lock model maps directly onto this narrative.
- **APM (Microsoft)** chose the literal name "Agent Package Manager" — confirming the category vocabulary even from a 200-pound gorilla. vd's positioning needs to share the category but distinguish on mechanism (vendoring vs declarative install).
- **Skills.sh (Vercel, Jan 2026 launch)** supports 18 agents and tracks install counts — a hosted registry model. vd's "you point at any URL, vendor it into your repo" model is the *opposite* design. The "vendoring" adjective communicates this opposition cleanly.
- **dbt deps** is the strongest structural precedent: vendoring + lockfile + multi-source + drift detection. dbt confidently calls itself a "package manager" and calls its units "packages" — but dbt got there before agent-skills existed and predates the "skill" convention. vd can't claim "package" as a unit without colliding with the more-recent ecosystem.

## Recommendation

**Pair:** `"skill"` (unit) + `"vendoring package manager"` (category with differentiating adjective).

**Why:**

- "Skill" is settled by Anthropic + OpenAI + every direct competitor + the SKILL.md filename. No upside in deviating.
- "Package manager" is the 2026 category for this exact problem space (per APM, Skilldex, JFrog, dbt analogy). Highest search recognition.
- "Vendoring" differentiates vd from the registry-flavored competitors (Skills.sh, AgentPkg, APM) by signaling "no host, files in your repo, SHA-locked." Maps to dbt-deps and Go-vendoring precedent. One adjective, zero new vocabulary.

**Runner-up:** `"skill"` + `"toolchain"` — wins only if vd ever wants to lean harder into the multi-target dispatch story (because "toolchain" implies multi-stage pipeline). Today, search recognition matters more.

**Avoid:** Inventing a unit noun ("kit," "recipe," "chart"). The ecosystem chose; fighting it costs more than it pays.

## Concrete Copy Recommendations

### GitHub repo description (~70 chars)

Current: `Package manager for coding agent skills (Claude Code, Codex)` (60 chars)

Recommended: `Vendoring package manager for coding-agent skills (Claude Code, Codex)` (70 chars)

The hyphen in "coding-agent" reads as a compound modifier and reduces "coding agent skills" ambiguity.

### Banner tagline (in `assets/banner.png`)

Current image text: `a package manager for coding agent skills`

Optional re-roll: `vendoring package manager for coding-agent skills` — one word longer, communicates the differentiator at the top.

Honest call: the banner is fine as-is. The differentiator can live one scroll-line down in the README intro. Re-rolling the banner just for one word costs a Codex generation and risks losing the current good composition.

### README intro paragraph

Current:
> **vd** is a single-binary package manager for the skills that power your coding agents.
> Vendor skills from any upstream, lock them with a SHA, sync them to every agent in your stack.

Recommended:
> **vd** is a single-binary vendoring package manager for the skills that power your coding agents.
> Skills live in your repo as plain files. `vd` fetches them from any upstream, locks them with a SHA, and dispatches them to every agent in your stack.

Two changes:
1. Add "vendoring" before "package manager" — the differentiation adjective.
2. Replace "lock them with a SHA, sync them to every agent" with a two-sentence flow that names the unique constraint ("skills live in your repo as plain files") before the action verbs. This is what separates vd from APM / Skills.sh, and it's worth one extra sentence.

### Goreleaser brew description

Current: `Package manager for coding agent skills (Claude Code, Codex)`

Recommended: `Vendoring package manager for coding-agent skills (Claude Code, Codex)`

### Section heading rename (optional)

Current: `## Supported agents`

Optional: `## Supported agent stacks` — slightly stronger framing of "stack" since agents combine into a stack. Not a strong opinion; either works.

## Logo / Visual Identity — Does It Need Updating?

No.

The current logo (vd monogram with arrow inside the `d` counter) and the banner (logo + wordmark + tagline + four accent dots) both align with the recommended positioning:

- The arrow visually represents "flow/sync/dispatch" — compatible with "vendoring + dispatch" framing.
- The four accent dots on the banner read as "multiple agents supported" — directly on-message for multi-target.
- The wordmark and monogram don't say "package manager" or "skill" explicitly — they're nouns-free, which means a positioning copy change doesn't invalidate them.

A reasonable refinement, but not required: re-roll the banner to swap the tagline to `vendoring package manager for coding-agent skills`. Cost: one Codex generation, risk of losing the current composition. Skip unless visual update is wanted for other reasons.

## Open Questions

- **Cursor / Cline support:** if vd adds a `.clinerules/` or `.cursorrules` target, those agents call their units "rules" — does vd describe them as "rules" or absorb under "skills"? Best path: support them as a sibling unit (`vd add ... --kind rule`), keep "skill" as the primary noun. Defer until concrete demand.
- **MCP server configs:** Anthropic's plugin layer includes MCP configs. If vd grows to manage MCP, the unit noun becomes ambiguous. Cleanest answer: vd manages the *plugin* (which contains skills + MCP + hooks), but skills remain the primary surface.
- **"skill" overload:** Salesforce, HR, gaming, and education all use "skill" generically. Search-engine cold scan for `vd skill` may surface unrelated content. Worth tracking: if SEO becomes a bottleneck, adding "coding-agent" as a permanent qualifier resolves disambiguation.

## References

- [Anthropic Claude Code Skills docs](https://code.claude.com/docs/en/skills) — the official "skill" terminology
- [OpenAI Codex Skills (Agent Skills open standard)](https://developers.openai.com/codex/skills) — joint convention with Anthropic
- [Codex AGENTS.md guide](https://developers.openai.com/codex/guides/agents-md) — project-level instructions standard (Google/OpenAI/Sourcegraph/Cursor/Factory joint launch)
- [Microsoft APM — Agent Package Manager](https://github.com/microsoft/apm) — direct competitor in the agent-skills package-manager space
- [AgentPkg — Agent Package Registry](https://www.agentpkg.com/) — registry-flavored competitor
- [Skilldex paper (arxiv 2604.16911)](https://arxiv.org/abs/2604.16911) — academic package-manager + registry for skills
- [JFrog: "Agent Skills are the New Packages of AI"](https://jfrog.com/blog/agent-skills-new-ai-packages/) — industry-narrative framing
- [Vercel Skills.sh launch coverage](https://www.buildmvpfast.com/blog/agent-skills-npm-ai-package-manager-2026) — registry supporting 18 agents
- [dbt deps docs](https://docs.getdbt.com/reference/commands/deps) — closest structural precedent to vd (vendoring + lockfile + multi-source)
- [Helm Glossary — why "chart" not "package"](https://helm.sh/docs/glossary/) — naming-the-unit precedent
- [Claude Code Skills vs Plugins (Morphllm)](https://www.morphllm.com/claude-code-skills-mcp-plugins) — skill-vs-plugin hierarchy
