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

/** Per-turn hook executions and skill invocations, rendered as chips (mirrors
 * the mock's hook-chips row). Claude-only: Codex emits neither, so a codex turn
 * renders an explicit "not emitted" line instead of an empty panel, which would
 * otherwise read as a parse failure. A hook over budget is a warning, so it
 * gets the amber warn tone, never the blue focal accent. */
export function HookTimeline({ hooks, skills, agent }: HookTimelineProps) {
  if (agent !== "claude-code") {
    return <p className="text-xs text-faint">hooks / skills: not emitted by {agent === "codex" ? "Codex" : agent}</p>;
  }
  if (hooks.length === 0 && skills.length === 0) return null;

  const sortedHooks = [...hooks].sort((a, b) => a.seq - b.seq);
  const sortedSkills = [...skills].sort((a, b) => a.seq - b.seq);

  return (
    <div className="grid gap-1.5">
      {sortedHooks.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {sortedHooks.map((h) => {
            const slow = h.event === "PreToolUse" && h.durationms > HOOK_BUDGET_MS;
            return (
              <span
                key={`${h.hookname}-${h.event}-${h.seq}`}
                className={cn(
                  "inline-flex items-center gap-1.5 rounded-sm border border-border bg-panel px-2 py-1 font-mono text-xs text-muted-foreground",
                  slow && "border-warn/60 text-warn",
                )}
              >
                {h.event} <b className={cn("text-foreground", slow && "text-warn")}>{h.hookname}</b>
                <span className={cn("text-ok", slow && "text-warn")}>{h.durationms}ms</span>
                {slow && <span>⚠ &gt;{HOOK_BUDGET_MS}ms budget</span>}
              </span>
            );
          })}
        </div>
      )}
      {sortedSkills.length > 0 && (
        <p className="text-xs text-faint">skill: {sortedSkills.map((s) => `/${s.name}`).join(" · ")}</p>
      )}
    </div>
  );
}
