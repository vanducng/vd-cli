import { Link } from "@tanstack/react-router";

import { AgentBadge } from "@/features/obs/components/agent-badge";
import { CostCell } from "@/features/obs/components/cost-cell";
import { TokenCell } from "@/features/obs/components/token-cell";
import { totalTokens, type SessionDetail } from "@/features/obs/schemas";
import { cn } from "@/lib/utils";

function StatTile({
  label,
  value,
  valueClassName,
}: {
  label: string;
  value: React.ReactNode;
  valueClassName?: string;
}) {
  return (
    <div className="rounded-md border border-border bg-panel px-4 py-3">
      <div className={cn("truncate text-xl tabular-nums", valueClassName)}>{value}</div>
      <span className="text-xs uppercase tracking-wide text-faint">{label}</span>
    </div>
  );
}

interface SessionHeaderProps {
  session: SessionDetail;
}

/** The portal's answer to `vd obs show`'s header block: id, agent, cwd/branch,
 * cli version, and roll-up stat tiles, with room to breathe instead of one
 * 80-column line. */
export function SessionHeader({ session }: SessionHeaderProps) {
  const tokens = totalTokens(session.tokens);
  const cacheHit = session.cachehitrate === null ? null : Math.round(session.cachehitrate * 100);

  return (
    <div className="mb-5">
      <div className="mb-4 flex items-start justify-between gap-4">
        <div className="min-w-0">
          <h1 className="truncate text-xl font-semibold">{session.title || "(untitled session)"}</h1>
          <p className="mt-0.5 flex flex-wrap items-center gap-2 font-mono text-sm text-muted-foreground">
            <span>{session.id}</span>
            <AgentBadge agent={session.agent} />
            {session.model && <span>{session.model}</span>}
            {session.cliversion && <span>cli {session.cliversion}</span>}
          </p>
        </div>
        <Link to="/obs/sessions" className="shrink-0 text-sm text-muted-foreground hover:underline">
          ‹ back
        </Link>
      </div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        <StatTile label="Turns" value={session.turncount} />
        <StatTile label="Tokens" value={<TokenCell tokens={tokens} />} />
        <StatTile
          label="Est cost"
          value={
            <>
              $<CostCell costUsd={session.costusd} model={session.model} />
            </>
          }
        />
        <StatTile label="Cache hit" value={cacheHit === null ? "?" : `${cacheHit}%`} />
        <StatTile
          label="cwd · branch"
          valueClassName="text-sm font-normal"
          value={
            <>
              <span className="block truncate" title={session.cwd}>
                {session.cwd || "?"}
              </span>
              {session.gitbranch && (
                <span className="block truncate text-xs text-muted-foreground">{session.gitbranch}</span>
              )}
            </>
          }
        />
      </div>
    </div>
  );
}
