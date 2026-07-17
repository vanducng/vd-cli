import type { HookExec, Skill } from "@/features/obs/schemas";
import { cn } from "@/lib/utils";

// vd's own convention (see decisions.md), not an upstream Claude Code budget:
// a PreToolUse hook over this is slow enough to be worth flagging in-line.
const HOOK_BUDGET_MS = 100;

interface HookTimelineProps {
  hooks: HookExec[];
  skills: Skill[];
  agent: string;
}

/** Per-turn hook executions and skill invocations. Claude-only: Codex emits
 * neither, so a codex turn renders an explicit "not emitted" line instead of
 * an empty panel, which would otherwise read as a parse failure. */
export function HookTimeline({ hooks, skills, agent }: HookTimelineProps) {
  if (agent !== "claude-code") {
    return <p className="text-xs text-faint">hooks / skills: not emitted by {agent === "codex" ? "Codex" : agent}</p>;
  }
  if (hooks.length === 0 && skills.length === 0) return null;

  const sortedHooks = [...hooks].sort((a, b) => a.seq - b.seq);
  const sortedSkills = [...skills].sort((a, b) => a.seq - b.seq);

  return (
    <div className="grid gap-0.5 text-xs text-faint">
      {sortedHooks.length > 0 && (
        <p>
          hooks:{" "}
          {sortedHooks.map((h, i) => {
            const slow = h.event === "PreToolUse" && h.durationms > HOOK_BUDGET_MS;
            return (
              <span key={`${h.hookname}-${h.event}-${h.seq}`} className={cn(slow && "text-primary")}>
                {i > 0 && " · "}
                {h.hookname} {h.event} {h.durationms}ms
                {slow && ` ⚠ >${HOOK_BUDGET_MS}ms budget`}
              </span>
            );
          })}
        </p>
      )}
      {sortedSkills.length > 0 && <p>skill: {sortedSkills.map((s) => `/${s.name}`).join(" · ")}</p>}
    </div>
  );
}
