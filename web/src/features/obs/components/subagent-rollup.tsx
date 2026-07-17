import { Link } from "@tanstack/react-router";

import { Badge } from "@/components/ui/badge";
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

  return (
    <div className="grid gap-1.5">
      {workflowId && <span className="text-xs text-faint">workflow {workflowId}</span>}
      <div className="flex flex-wrap gap-2">
        {spans.map((s) => (
          <RollupBadge key={s.id} span={s} />
        ))}
      </div>
    </div>
  );
}

function RollupBadge({ span }: { span: ToolSpan }) {
  const content = (
    <span className="inline-flex items-center gap-2 rounded-pill border border-border bg-panel-2 px-3 py-0.5 text-sm">
      <Badge>{span.subagentname}</Badge>
      {span.rolluptokens ? (
        <span className="tabular-nums text-muted-foreground">
          {totalTokens(span.rolluptokens).toLocaleString()} tok
        </span>
      ) : (
        <span className="text-faint">rollup pending</span>
      )}
      {span.rollupcostusd != null && (
        <span className="tabular-nums text-muted-foreground">${span.rollupcostusd.toFixed(2)}</span>
      )}
    </span>
  );

  if (!span.subagentsessionid) return content;
  return (
    <Link to="/obs/sessions/$id" params={{ id: span.subagentsessionid }} className="hover:opacity-80">
      {content}
    </Link>
  );
}
