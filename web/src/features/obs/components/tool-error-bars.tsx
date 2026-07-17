import { Skeleton } from "@/components/ui/skeleton";
import { formatCount } from "@/features/obs/lib/format";
import type { ToolErrorCount } from "@/features/obs/schemas";

const TOP_N = 8;
const SKELETON_ROWS = 5;

interface ToolErrorBarsProps {
  items: ToolErrorCount[];
  isLoading?: boolean;
  error?: Error | null;
}

/** Top-8 tools by error count as a simple proportional bar list — the cluster
 * table is the centerpiece, this is a scan-first summary above it. */
export function ToolErrorBars({ items, isLoading, error }: ToolErrorBarsProps) {
  if (error) {
    return <div className="rounded-md border border-border bg-panel p-4 text-sm text-err">{error.message}</div>;
  }

  if (isLoading && items.length === 0) {
    return (
      <div className="grid gap-2.5 rounded-md border border-border bg-panel p-4">
        {Array.from({ length: SKELETON_ROWS }).map((_, i) => (
          <Skeleton key={i} className="h-5 w-full" />
        ))}
      </div>
    );
  }

  const top = items.slice(0, TOP_N);

  if (top.length === 0) {
    return (
      <div className="rounded-md border border-border bg-panel p-4 text-center text-sm text-muted-foreground">
        No tool errors in this window.
      </div>
    );
  }

  const max = Math.max(...top.map((t) => t.count));

  return (
    <div className="grid gap-2.5 rounded-md border border-border bg-panel p-4">
      {top.map((t) => (
        <div key={t.tool} className="flex items-center gap-3">
          <span className="w-40 shrink-0 truncate font-mono text-xs text-muted-foreground">{t.tool}</span>
          <div className="h-2 flex-1 overflow-hidden rounded-pill bg-panel-2">
            <div className="h-full rounded-pill bg-err/70" style={{ width: `${(t.count / max) * 100}%` }} />
          </div>
          <span className="w-14 shrink-0 text-right text-xs tabular-nums text-muted-foreground">{formatCount(t.count)}</span>
        </div>
      ))}
    </div>
  );
}
