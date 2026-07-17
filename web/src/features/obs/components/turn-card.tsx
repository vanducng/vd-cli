import { useState } from "react";

import { CostCell } from "@/features/obs/components/cost-cell";
import { HookTimeline } from "@/features/obs/components/hook-timeline";
import { SubagentRollup } from "@/features/obs/components/subagent-rollup";
import { TokenCell } from "@/features/obs/components/token-cell";
import { ToolSpanSummary } from "@/features/obs/components/tool-span-block";
import { formatClock, formatDuration } from "@/features/obs/lib/format";
import { cacheHitRate, totalTokens, type Turn } from "@/features/obs/schemas";
import { cn } from "@/lib/utils";

const COLLAPSE_THRESHOLD = 400;

function Bubble({ role, text }: { role: "user" | "assistant"; text: string }) {
  const long = text.length > COLLAPSE_THRESHOLD;
  const [expanded, setExpanded] = useState(!long);

  if (!text) {
    if (role === "assistant") return null;
    return (
      <div className="rounded-md border border-dashed border-border bg-panel-2/50 px-4 py-3 text-sm italic text-faint">
        agent-continued turn (no user prompt)
      </div>
    );
  }

  return (
    <div
      className={cn(
        "whitespace-pre-wrap rounded-md border px-4 py-3 text-sm leading-relaxed",
        role === "user" ? "border-border bg-panel-2 text-muted-foreground" : "border-primary/25 bg-primary/[0.08] text-foreground",
      )}
    >
      <span
        className={cn(
          "mb-1.5 block font-mono text-xs font-bold uppercase tracking-wide",
          role === "user" ? "text-faint" : "text-primary",
        )}
      >
        {role}
      </span>
      {expanded ? text : `${text.slice(0, COLLAPSE_THRESHOLD)}…`}
      {long && (
        <button
          type="button"
          className="ml-2 whitespace-nowrap text-xs text-primary hover:underline"
          onClick={() => setExpanded((v) => !v)}
        >
          {expanded ? "show less" : "show more"}
        </button>
      )}
    </div>
  );
}

interface TurnCardProps {
  turn: Turn;
  agent: string;
  sessionWorkflowId?: string;
  isLast?: boolean;
}

/** One turn in the transcript timeline: a left gutter (clock, turn index,
 * duration, tokens, cost, cache%) connected to the next turn by a rail line,
 * and the turn's content — prompt/response bubbles, tool calls, hooks, and
 * subagent rollups — on the right. */
export function TurnCard({ turn, agent, sessionWorkflowId, isLast }: TurnCardProps) {
  const tokens = totalTokens(turn.tokens);
  const cachePct = cacheHitRate(turn.tokens);
  const subagentSpans = turn.toolspans.filter((s) => s.subagentname);
  const regularSpans = turn.toolspans.filter((s) => !s.subagentname);

  return (
    <li className="grid grid-cols-[92px_1fr] gap-3 sm:grid-cols-[132px_1fr] sm:gap-4">
      <div className="relative pl-5">
        {!isLast && <span className="absolute bottom-[-20px] left-[9px] top-2 w-px bg-border" aria-hidden />}
        <span
          className="absolute left-0 top-1.5 h-3 w-3 rounded-pill bg-primary shadow-[0_0_0_4px_hsl(var(--primary)/0.22)]"
          aria-hidden
        />
        <span className="block font-mono text-sm font-semibold text-foreground">{formatClock(turn.startedat)}</span>
        <span className="mt-1 block font-mono text-xs leading-relaxed text-faint">
          turn {turn.index + 1} · {formatDuration(turn.durationms)}
          <br />
          <TokenCell tokens={tokens} /> tok · $<CostCell costUsd={turn.costusd} model={turn.model} />
          <br />
          {cachePct === null ? "? cache" : `${Math.round(cachePct * 100)}% cache`}
        </span>
      </div>
      <div className="mb-1 grid gap-3 rounded-md border border-border bg-panel p-4">
        <Bubble role="user" text={turn.prompttext} />
        <Bubble role="assistant" text={turn.responsetext} />
        <ToolSpanSummary spans={regularSpans} />
        <HookTimeline hooks={turn.hookexecs} skills={turn.skills} agent={agent} />
        <SubagentRollup spans={subagentSpans} workflowId={sessionWorkflowId} />
      </div>
    </li>
  );
}
