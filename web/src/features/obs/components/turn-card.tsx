import { useState } from "react";

import { CostCell } from "@/features/obs/components/cost-cell";
import { HookTimeline } from "@/features/obs/components/hook-timeline";
import { SubagentRollup } from "@/features/obs/components/subagent-rollup";
import { TokenCell } from "@/features/obs/components/token-cell";
import { ToolSpanSummary } from "@/features/obs/components/tool-span-block";
import { totalTokens, type Turn } from "@/features/obs/schemas";
import { cn } from "@/lib/utils";

const COLLAPSE_THRESHOLD = 400;

function formatClock(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function formatDuration(ms: number): string {
  const totalSec = Math.round(ms / 1000);
  const m = Math.floor(totalSec / 60);
  const s = totalSec % 60;
  return m === 0 ? `${s}s` : `${m}m${String(s).padStart(2, "0")}s`;
}

function Bubble({ role, text }: { role: "user" | "assistant"; text: string }) {
  const long = text.length > COLLAPSE_THRESHOLD;
  const [expanded, setExpanded] = useState(!long);
  if (!text) return null;

  return (
    <div
      className={cn(
        "whitespace-pre-wrap rounded-md border border-border bg-panel px-4 py-3 text-sm leading-relaxed",
        role === "user" ? "border-l-[3px] border-l-info" : "border-l-[3px] border-l-claude text-muted-foreground",
      )}
    >
      {expanded ? text : `${text.slice(0, COLLAPSE_THRESHOLD)}…`}
      {long && (
        <button
          type="button"
          className="ml-2 whitespace-nowrap text-xs text-info hover:underline"
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
}

/** One user prompt and everything the agent did in response: header stats,
 * prompt/response bubbles, tool calls, hook timeline, and subagent rollups. */
export function TurnCard({ turn, agent, sessionWorkflowId }: TurnCardProps) {
  const tokens = totalTokens(turn.tokens);
  const subagentSpans = turn.toolspans.filter((s) => s.subagentname);
  const regularSpans = turn.toolspans.filter((s) => !s.subagentname);

  return (
    <div className="mb-3 overflow-hidden rounded-md border border-border">
      <div className="flex flex-wrap items-baseline gap-3 bg-panel-2 px-4 py-2 text-sm text-muted-foreground">
        <b className="text-foreground">turn {turn.index + 1}</b>
        <span className="font-mono">{formatClock(turn.startedat)}</span>
        <span>{formatDuration(turn.durationms)}</span>
        <span>
          <TokenCell tokens={tokens} /> tok
        </span>
        <span>
          $<CostCell costUsd={turn.costusd} model={turn.model} />
        </span>
      </div>
      <div className="grid gap-2 px-4 py-3">
        <Bubble role="user" text={turn.prompttext} />
        <Bubble role="assistant" text={turn.responsetext} />
        <ToolSpanSummary spans={regularSpans} />
        <HookTimeline hooks={turn.hookexecs} skills={turn.skills} agent={agent} />
        <SubagentRollup spans={subagentSpans} workflowId={sessionWorkflowId} />
      </div>
    </div>
  );
}
