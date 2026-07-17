import { useState } from "react";

import { formatMs } from "@/features/obs/lib/format";
import type { ToolSpan } from "@/features/obs/schemas";

interface ToolSpanBlockProps {
  span: ToolSpan;
}

/** One tool invocation. A failed call defaults expanded, since that is what you
 * are scrolling to find, and renders as a red-bordered block; a successful call
 * collapses its input/output behind a toggle so 25 identical Bash calls don't
 * drown the turn. */
export function ToolSpanBlock({ span }: ToolSpanBlockProps) {
  const [expanded, setExpanded] = useState(!span.ok);
  const hasBody = Boolean(span.input || span.output || span.error);

  if (!span.ok) {
    return (
      <div className="overflow-hidden rounded-sm border border-err/40 bg-err/[0.08]">
        <button
          type="button"
          className="flex w-full items-center gap-2 px-3 py-2 text-left font-mono text-xs font-semibold text-err"
          onClick={() => hasBody && setExpanded((v) => !v)}
          disabled={!hasBody}
        >
          <span className="truncate">{span.name} — failed</span>
          <span className="ml-auto shrink-0 font-normal text-err/80">{formatMs(span.durationms)}</span>
        </button>
        {expanded && hasBody && (
          <div className="grid gap-1 border-t border-err/30 bg-panel p-2 text-xs">
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

  return (
    <div className="border-l-2 border-border pl-3 font-mono text-sm text-muted-foreground">
      <button
        type="button"
        className="flex w-full items-center gap-2 py-1 text-left"
        onClick={() => hasBody && setExpanded((v) => !v)}
        disabled={!hasBody}
      >
        <span className="truncate">{span.name}</span>
        <span className="ml-auto shrink-0 text-xs text-faint">{formatMs(span.durationms)}</span>
      </button>
      {expanded && hasBody && (
        <div className="grid gap-1 pb-2 text-xs">
          {span.input && (
            <pre className="max-h-40 overflow-auto whitespace-pre-wrap break-words rounded-sm bg-panel-2 p-2 text-muted-foreground">
              {span.input}
            </pre>
          )}
          {span.output && (
            <pre className="max-h-60 overflow-auto whitespace-pre-wrap break-words rounded-sm bg-panel-2 p-2 text-muted-foreground">
              {span.output}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}

interface ToolSpanSummaryProps {
  spans: ToolSpan[];
}

/** A turn's tool calls, compact by default: successful calls roll up into one
 * chip reading "Name ×N · Name ×N" (mirrors the CLI mock's "Bash ×25 · Read ×4"),
 * click to expand the full list. Failures always render their own expanded
 * block beneath the rollup chip, regardless of the toggle. */
export function ToolSpanSummary({ spans }: ToolSpanSummaryProps) {
  const [showAll, setShowAll] = useState(false);
  if (spans.length === 0) return null;

  const failed = spans.filter((s) => !s.ok);
  const ok = spans.filter((s) => s.ok);
  const counts = new Map<string, number>();
  for (const s of ok) counts.set(s.name, (counts.get(s.name) ?? 0) + 1);

  return (
    <div className="grid gap-1.5">
      {ok.length > 0 && (
        <div className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            className="flex items-center gap-2 rounded-sm border border-border bg-panel px-3 py-1.5 font-mono text-xs text-muted-foreground hover:border-primary/50 hover:text-foreground"
            onClick={() => setShowAll((v) => !v)}
          >
            <span className="font-bold uppercase tracking-wide text-faint">tools</span>
            {[...counts.entries()].map(([name, n]) => `${name} ×${n}`).join(" · ")}
          </button>
          {failed.length > 0 && (
            <span className="text-xs font-semibold text-err">
              {failed.length} error{failed.length === 1 ? "" : "s"}
            </span>
          )}
        </div>
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
