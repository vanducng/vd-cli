import type { ReactNode } from "react";
import { Link } from "@tanstack/react-router";

import { AgentBadge } from "@/features/obs/components/agent-badge";
import { CostCell } from "@/features/obs/components/cost-cell";
import { TokenCell } from "@/features/obs/components/token-cell";
import { formatStarted } from "@/features/obs/lib/format";
import { totalTokens, type SessionDetail } from "@/features/obs/schemas";

function Stat({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="min-w-[110px] rounded-md border border-border bg-panel px-4 py-2.5">
      <div className="text-xs font-semibold uppercase tracking-wide text-faint">{label}</div>
      <div className="mt-1 truncate text-xl font-semibold tabular-nums">{value}</div>
    </div>
  );
}

interface SessionHeaderProps {
  session: SessionDetail;
}

/** Compact session header: agent + title + raw id, a fact line (cwd/branch/model/
 * started), and one totals row (turns/tokens/cost/cache hit). Totals render only
 * here — no separate stat-card grid — so the two can never disagree. */
export function SessionHeader({ session }: SessionHeaderProps) {
  const tokens = totalTokens(session.tokens);
  const cacheHit = session.cachehitrate === null ? null : Math.round(session.cachehitrate * 100);

  return (
    <div className="mb-6 rounded-lg border border-border bg-panel p-5">
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-3">
            <AgentBadge agent={session.agent} />
            <h1 className="truncate text-lg font-semibold text-foreground">
              {session.title || "untitled session"}
            </h1>
          </div>
          <p className="mt-1 truncate font-mono text-xs text-faint" title={session.id}>
            {session.id}
          </p>
          <p className="mt-3 flex flex-wrap gap-x-5 gap-y-1 font-mono text-xs text-faint">
            <span>
              <b className="text-muted-foreground">cwd</b> {session.cwd || "?"}
            </span>
            {session.gitbranch && (
              <span>
                <b className="text-muted-foreground">branch</b> {session.gitbranch}
              </span>
            )}
            {session.model && (
              <span>
                <b className="text-muted-foreground">model</b> {session.model}
              </span>
            )}
            <span>
              <b className="text-muted-foreground">started</b> {formatStarted(session.startedat)}
            </span>
          </p>
        </div>
        <Link to="/obs/sessions" className="shrink-0 text-sm text-muted-foreground hover:underline">
          ‹ back
        </Link>
      </div>

      <div className="mt-4 flex flex-wrap gap-3">
        <Stat label="Turns" value={session.turncount} />
        <Stat label="Tokens" value={<TokenCell tokens={tokens} />} />
        <Stat
          label="Cost"
          value={
            <>
              $<CostCell costUsd={session.costusd} model={session.model} />
            </>
          }
        />
        <Stat label="Cache hit" value={cacheHit === null ? "?" : `${cacheHit}%`} />
      </div>
    </div>
  );
}
