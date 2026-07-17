import { Link } from "@tanstack/react-router";

import { formatUsd } from "@/features/obs/lib/format";
import { totalTokens, type ToolSpan } from "@/features/obs/schemas";

interface SubagentRollupProps {
  spans: ToolSpan[];
  workflowId?: string;
}

/** One badge per subagent spawned in this turn: rolled-up tokens/$, linking to
 * the subagent's own session when one exists. When the whole session carries a
 * workflowid, every rollup in it belongs to that one workflow, so the group
 * gets a single label rather than repeating it per badge. Tokens/cost here are
 * display-only, already counted on the subagent's own session, so summing
 * them into this turn's total would double-count. */
export function SubagentRollup({ spans, workflowId }: SubagentRollupProps) {
  if (spans.length === 0) return null;

  // identical name+pending pills collapse to one "×N" — two bare "debugger
  // rollup pending" chips side by side read as a render bug, not two spawns
  const pending = new Map<string, number>();
  const resolved: ToolSpan[] = [];
  for (const s of spans) {
    if (!s.rolluptokens && !s.subagentsessionid) {
      const name = s.subagentname ?? "subagent";
      pending.set(name, (pending.get(name) ?? 0) + 1);
    } else {
      resolved.push(s);
    }
  }

  return (
    <div className="grid gap-1.5">
      {workflowId && <span className="text-xs text-faint">workflow {workflowId}</span>}
      <div className="flex flex-wrap gap-2">
        {resolved.map((s) => (
          <RollupBadge key={s.id} span={s} />
        ))}
        {[...pending.entries()].map(([name, n]) => (
          <span
            key={name}
            className="inline-flex items-center gap-2 rounded-pill border border-codex/40 bg-codex/10 px-3 py-1 font-mono text-xs text-codex"
          >
            <b className="font-bold">
              {name}
              {n > 1 && ` ×${n}`}
            </b>
            <span className="text-faint">rollup pending</span>
          </span>
        ))}
      </div>
    </div>
  );
}

function RollupBadge({ span }: { span: ToolSpan }) {
  const content = (
    <span className="inline-flex items-center gap-2 rounded-pill border border-codex/40 bg-codex/10 px-3 py-1 font-mono text-xs text-codex">
      <b className="font-bold">{span.subagentname}</b>
      {span.rolluptokens ? (
        <span className="tabular-nums text-muted-foreground">
          {totalTokens(span.rolluptokens).toLocaleString()} tok
        </span>
      ) : (
        <span className="text-faint">rollup pending</span>
      )}
      {span.rollupcostusd != null && (
        <span className="tabular-nums text-muted-foreground">{formatUsd(span.rollupcostusd)}</span>
      )}
      {span.subagentsessionid && <span aria-hidden>↗</span>}
    </span>
  );

  if (!span.subagentsessionid) return content;
  return (
    <Link to="/obs/sessions/$id" params={{ id: span.subagentsessionid }} className="hover:opacity-80">
      {content}
    </Link>
  );
}
