import { useState } from "react";

import { formatMs } from "@/features/obs/lib/format";
import type { ToolSpan } from "@/features/obs/schemas";
import { cn } from "@/lib/utils";

interface ToolSpanBlockProps {
  span: ToolSpan;
}

/** One tool invocation. A failed call defaults expanded, since that is what you
 * are scrolling to find; a successful call collapses its input/output behind a
 * toggle so 25 identical Bash calls don't drown the turn. */
export function ToolSpanBlock({ span }: ToolSpanBlockProps) {
  const [expanded, setExpanded] = useState(!span.ok);
  const hasBody = Boolean(span.input || span.output || span.error);

  return (
    <div
      className={cn(
        "border-l-2 pl-3 font-mono text-sm",
        span.ok ? "border-border text-muted-foreground" : "border-err text-err",
      )}
    >
      <button
        type="button"
        className={cn("flex w-full items-center gap-2 py-1 text-left", !hasBody && "cursor-default")}
        onClick={() => hasBody && setExpanded((v) => !v)}
      >
        <span className="truncate">{span.name}</span>
        {!span.ok && <span className="shrink-0">failed</span>}
        <span className="ml-auto shrink-0 text-xs text-faint">{formatMs(span.durationms)}</span>
      </button>
      {expanded && hasBody && (
        <div className="grid gap-1 pb-2 text-xs">
          {span.input && (
            <pre className="max-h-40 overflow-auto whitespace-pre-wrap break-words rounded-sm bg-panel-2 p-2 text-muted-foreground">
              {span.input}
            </pre>
          )}
          {span.error ? (
            <pre className="max-h-60 overflow-auto whitespace-pre-wrap break-words rounded-sm bg-panel-2 p-2 text-err">
              {span.error}
            </pre>
          ) : (
            span.output && (
              <pre className="max-h-60 overflow-auto whitespace-pre-wrap break-words rounded-sm bg-panel-2 p-2 text-muted-foreground">
                {span.output}
              </pre>
            )
          )}
        </div>
      )}
    </div>
  );
}

interface ToolSpanSummaryProps {
  spans: ToolSpan[];
}

/** A turn's tool calls, compact by default: successful calls roll up into
 * "Name ×N" counts (mirrors the CLI mock's "Bash ×25 · Read ×4 · Grep ×2"),
 * click to expand the full list. Failures always render their own expanded
 * block beneath the summary, regardless of the toggle. */
export function ToolSpanSummary({ spans }: ToolSpanSummaryProps) {
  const [showAll, setShowAll] = useState(false);
  if (spans.length === 0) return null;

  const failed = spans.filter((s) => !s.ok);
  const ok = spans.filter((s) => s.ok);
  const counts = new Map<string, number>();
  for (const s of ok) counts.set(s.name, (counts.get(s.name) ?? 0) + 1);

  return (
    <div className="grid gap-1">
      {ok.length > 0 && (
        <button
          type="button"
          className="w-fit font-mono text-sm text-muted-foreground hover:text-foreground"
          onClick={() => setShowAll((v) => !v)}
        >
          {[...counts.entries()].map(([name, n]) => `${name} ×${n}`).join(" · ")}
        </button>
      )}
      {showAll && (
        <div className="grid gap-1">
          {ok.map((s) => (
            <ToolSpanBlock key={s.id} span={s} />
          ))}
        </div>
      )}
      {failed.map((s) => (
        <ToolSpanBlock key={s.id} span={s} />
      ))}
    </div>
  );
}
