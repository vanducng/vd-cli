import { Fragment, useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import { ChevronDown, ChevronRight } from "lucide-react";

import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { PaginationBar } from "@/features/shared/components/data-table";
import { CopyButton } from "@/features/obs/components/copy-button";
import { formatCount } from "@/features/obs/lib/format";
import type { ErrorCluster, Trend } from "@/features/obs/schemas";
import { cn } from "@/lib/utils";

const COLS = 7;
const PAGE_SIZE = 25;
const SKILL_CHIP_LIMIT = 3;
const SKELETON_ROWS = 10;

function TrendChip({ trend }: { trend: Trend }) {
  switch (trend) {
    case "up":
      return <span className="rounded-pill border border-err/40 px-2 py-0.5 text-xs text-err">↑ up</span>;
    case "down":
      return <span className="rounded-pill border border-ok/40 px-2 py-0.5 text-xs text-ok">↓ down</span>;
    case "flat":
      return <span className="rounded-pill border border-border px-2 py-0.5 text-xs text-faint">→ flat</span>;
    case "low sample":
      return <span className="rounded-pill border border-border px-2 py-0.5 text-xs italic text-faint">low sample</span>;
    default:
      return <span className="text-xs text-faint">—</span>;
  }
}

function buildInvestigationPrompt(cluster: ErrorCluster): string {
  const first = cluster.evidence[0];
  const tools = cluster.affectedtools.join(", ") || "(none recorded)";
  const evidenceLine = first
    ? `Evidence: vd obs show ${first.sessionid} --json (turn ${first.turnindex}).`
    : "Evidence: no sample session recorded.";
  const skillLine = cluster.suggestedfocus ? ` Skill file: ${cluster.suggestedfocus}` : "";
  return `Investigate this recurring agent error cluster: signature "${cluster.signature}". ${cluster.count} occurrences, tools: ${tools}. ${evidenceLine}${skillLine}`;
}

function ClusterDetail({ cluster }: { cluster: ErrorCluster }) {
  return (
    <div className="grid gap-4">
      <div>
        <div className="mb-1 text-xs font-semibold uppercase tracking-wide text-faint">Sample</div>
        <pre className="max-h-60 overflow-auto whitespace-pre-wrap break-words rounded-sm bg-panel-2 p-3 font-mono text-xs text-muted-foreground">
          {cluster.sample}
        </pre>
      </div>

      {cluster.suggestedfocus && (
        <div className="flex items-center justify-between gap-3 rounded-sm border border-primary/40 bg-primary/[0.06] px-3 py-2">
          <div className="min-w-0">
            <div className="text-xs font-semibold uppercase tracking-wide text-primary">Suggested focus</div>
            <div className="truncate font-mono text-xs text-muted-foreground">{cluster.suggestedfocus}</div>
          </div>
          <CopyButton value={cluster.suggestedfocus} label="Copy path" />
        </div>
      )}

      {cluster.cooccurringskills.length > 0 && (
        <div>
          <div className="mb-1 text-xs font-semibold uppercase tracking-wide text-faint">Co-occurring skills</div>
          <div className="flex flex-wrap gap-2">
            {cluster.cooccurringskills.map((s) => (
              <div key={s.name} className="flex items-center gap-1.5 rounded-pill border border-border py-0.5 pl-2 pr-0.5">
                <span className="text-xs text-muted-foreground">{s.name}</span>
                <CopyButton value={s.path} label="path" className="h-6 px-1.5" />
              </div>
            ))}
          </div>
        </div>
      )}

      {cluster.evidence.length > 0 && (
        <div>
          <div className="mb-1 text-xs font-semibold uppercase tracking-wide text-faint">
            Evidence ({cluster.evidence.length})
          </div>
          <div className="flex max-h-40 flex-wrap gap-2 overflow-auto">
            {cluster.evidence.map((e) => (
              <Link
                key={e.turnid}
                to="/obs/sessions/$id"
                params={{ id: e.sessionid }}
                className="rounded-pill border border-border px-2 py-1 font-mono text-xs text-info hover:underline"
              >
                turn {e.turnindex}
              </Link>
            ))}
          </div>
        </div>
      )}

      <div>
        <CopyButton value={buildInvestigationPrompt(cluster)} label="Copy investigation prompt" />
      </div>
    </div>
  );
}

interface HealthClustersTableProps {
  clusters: ErrorCluster[];
  isLoading?: boolean;
  error?: Error | null;
}

/** The health view's centerpiece: signature, count, trend, tools, skills,
 * sessions — expand a row for the raw sample, evidence links, co-occurring
 * skills, and an investigation-prompt template. Count and trend reliability
 * are separate cues (a row can read count=377, trend="low sample"), so the
 * row and its count never dim for an unreliable trend — only the chip does.
 * Client-paged like a sibling built on the shared PaginationBar (clusters can
 * run into the thousands, but need a custom expand-row per row, which the
 * generic column-def DataTable doesn't support). */
export function HealthClustersTable({ clusters, isLoading, error }: HealthClustersTableProps) {
  const [expanded, setExpanded] = useState<Set<number>>(new Set());
  const [page, setPage] = useState(0);

  const pageClusters = useMemo(
    () => clusters.slice(page * PAGE_SIZE, page * PAGE_SIZE + PAGE_SIZE),
    [clusters, page],
  );

  function toggle(index: number) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(index)) next.delete(index);
      else next.add(index);
      return next;
    });
  }

  const showSkeleton = isLoading && clusters.length === 0;

  return (
    <div className="flex flex-col gap-3">
      <div className="max-h-[75vh] overflow-auto rounded-md border border-border bg-panel">
        <Table>
          <TableHeader className="sticky top-0 z-10 bg-panel">
            <TableRow className="hover:bg-transparent">
              <TableHead className="w-8" />
              <TableHead>Signature</TableHead>
              <TableHead className="text-right">Count</TableHead>
              <TableHead>Trend</TableHead>
              <TableHead>Tools</TableHead>
              <TableHead>Skills</TableHead>
              <TableHead className="text-right">Sessions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {error ? (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={COLS} className="h-[420px] text-center text-err">
                  {error.message}
                </TableCell>
              </TableRow>
            ) : showSkeleton ? (
              Array.from({ length: SKELETON_ROWS }).map((_, i) => (
                <TableRow key={i} className="hover:bg-transparent">
                  {Array.from({ length: COLS }).map((_, j) => (
                    <TableCell key={j}>
                      <Skeleton className="h-4 w-full max-w-[160px]" />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : clusters.length === 0 ? (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={COLS} className="h-[420px] text-center text-muted-foreground">
                  No tool errors in this window.
                </TableCell>
              </TableRow>
            ) : (
              pageClusters.map((cluster, i) => {
                const index = page * PAGE_SIZE + i;
                const isOpen = expanded.has(index);
                return (
                  <Fragment key={index}>
                    <TableRow
                      className={cn("cursor-pointer even:bg-panel-2/30 hover:bg-panel-2", isOpen && "bg-panel-2")}
                      onClick={() => toggle(index)}
                    >
                      <TableCell>
                        {isOpen ? (
                          <ChevronDown className="h-3.5 w-3.5 text-faint" />
                        ) : (
                          <ChevronRight className="h-3.5 w-3.5 text-faint" />
                        )}
                      </TableCell>
                      <TableCell className="max-w-[420px] truncate font-mono text-xs">{cluster.signature}</TableCell>
                      <TableCell className="text-right tabular-nums font-semibold">{formatCount(cluster.count)}</TableCell>
                      <TableCell>
                        <TrendChip trend={cluster.trend} />
                      </TableCell>
                      <TableCell className="max-w-[200px] truncate font-mono text-xs text-muted-foreground">
                        {cluster.affectedtools.join(", ") || "—"}
                      </TableCell>
                      <TableCell className="max-w-[220px]">
                        {cluster.cooccurringskills.length === 0 ? (
                          <span className="text-xs text-faint">—</span>
                        ) : (
                          <div className="flex flex-wrap gap-1">
                            {cluster.cooccurringskills.slice(0, SKILL_CHIP_LIMIT).map((s) => (
                              <span
                                key={s.name}
                                className="rounded-pill border border-border px-1.5 py-0.5 text-xs text-muted-foreground"
                              >
                                {s.name}
                              </span>
                            ))}
                            {cluster.cooccurringskills.length > SKILL_CHIP_LIMIT && (
                              <span className="text-xs text-faint">
                                +{cluster.cooccurringskills.length - SKILL_CHIP_LIMIT}
                              </span>
                            )}
                          </div>
                        )}
                      </TableCell>
                      <TableCell className="text-right tabular-nums text-muted-foreground">
                        {cluster.sessions.length}
                      </TableCell>
                    </TableRow>
                    {isOpen && (
                      <TableRow className="hover:bg-transparent">
                        <TableCell colSpan={COLS} className="bg-background/40 p-4">
                          <ClusterDetail cluster={cluster} />
                        </TableCell>
                      </TableRow>
                    )}
                  </Fragment>
                );
              })
            )}
          </TableBody>
        </Table>
      </div>

      {clusters.length > PAGE_SIZE && (
        <PaginationBar
          offset={page * PAGE_SIZE}
          pageSize={PAGE_SIZE}
          total={clusters.length}
          canPrev={page > 0}
          canNext={(page + 1) * PAGE_SIZE < clusters.length}
          onPrev={() => setPage((p) => Math.max(0, p - 1))}
          onNext={() => setPage((p) => p + 1)}
        />
      )}
    </div>
  );
}
