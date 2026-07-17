import type { ReactNode } from "react";
import { Link } from "@tanstack/react-router";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { PaginationBar } from "@/features/shared/components/data-table";
import { AgentBadge } from "@/features/obs/components/agent-badge";
import { CostCell } from "@/features/obs/components/cost-cell";
import { TokenCell } from "@/features/obs/components/token-cell";
import { formatStarted } from "@/features/obs/lib/format";
import type { SessionSummary } from "@/features/obs/schemas";
import { cn } from "@/lib/utils";

function totalTokens(s: SessionSummary): number {
  return s.tokens.input + s.tokens.output + s.tokens.cacheread + s.tokens.cachewrite;
}

const COLS = 8;

interface SessionsTableProps {
  sessions: SessionSummary[];
  isLoading?: boolean;
  error?: Error | null;
  isFiltered?: boolean;
  onClearFilters?: () => void;
  toolbar?: ReactNode;
  pageSize: number;
  offset: number;
  total: number;
  onPrev: () => void;
  onNext: () => void;
}

/** Server-paged sessions table: the API already returns one page (limit/offset),
 * so this renders raw rows instead of routing through the shared client-paged
 * DataTable, which would otherwise show a second, wrong "1-N of N" footer for
 * the current page alongside the real total from the server. */
export function SessionsTable({
  sessions,
  isLoading,
  error,
  isFiltered,
  onClearFilters,
  toolbar,
  pageSize,
  offset,
  total,
  onPrev,
  onNext,
}: SessionsTableProps) {
  const showSkeleton = isLoading && sessions.length === 0;

  return (
    <div className="flex flex-col gap-3">
      {toolbar}

      {/* sticky thead binds to the nearest scroll container, so that container
          must own vertical scroll too — viewport-sticky inside overflow-x breaks */}
      <div className="max-h-[75vh] overflow-auto rounded-md border border-border bg-panel">
        <Table className="min-w-[760px]">
          <TableHeader className="sticky top-0 z-10 bg-panel">
            <TableRow className="hover:bg-transparent">
              <TableHead>Started</TableHead>
              <TableHead>Agent</TableHead>
              <TableHead>Title</TableHead>
              <TableHead>Project</TableHead>
              <TableHead>Model</TableHead>
              <TableHead className="text-right">Turns</TableHead>
              <TableHead className="text-right">Tokens</TableHead>
              <TableHead className="text-right">Est $</TableHead>
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
              Array.from({ length: pageSize }).map((_, i) => (
                <TableRow key={i} className="hover:bg-transparent">
                  {Array.from({ length: COLS }).map((_, j) => (
                    <TableCell key={j}>
                      <Skeleton className="h-4 w-full max-w-[160px]" />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : sessions.length === 0 ? (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={COLS} className="h-[420px] text-center align-middle">
                  <div className="flex h-full flex-col items-center justify-center gap-2 text-muted-foreground">
                    <p className="text-sm">
                      {isFiltered ? "No sessions match the current filters." : "No sessions synced yet."}
                    </p>
                    {isFiltered && onClearFilters && (
                      <Button variant="outline" size="sm" onClick={onClearFilters}>
                        Clear filters
                      </Button>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            ) : (
              sessions.map((s) => (
                <TableRow key={s.id} className="even:bg-panel-2/30 hover:bg-panel-2">
                  <TableCell className="font-mono text-xs text-muted-foreground">
                    {formatStarted(s.startedat)}
                  </TableCell>
                  <TableCell>
                    <AgentBadge agent={s.agent} />
                  </TableCell>
                  <TableCell>
                    <Link
                      to="/obs/sessions/$id"
                      params={{ id: s.id }}
                      className={cn("hover:underline", s.title ? "font-medium text-foreground" : "italic text-faint")}
                    >
                      {s.title || "untitled session"}
                    </Link>
                  </TableCell>
                  <TableCell className="font-mono text-xs text-faint">{s.project || "—"}</TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">{s.model}</TableCell>
                  <TableCell className="text-right tabular-nums">{s.turncount}</TableCell>
                  <TableCell className="text-right">
                    <TokenCell tokens={totalTokens(s)} />
                  </TableCell>
                  <TableCell className="text-right">
                    <CostCell costUsd={s.costusd} model={s.model} />
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {total > 0 && (
        <PaginationBar
          offset={offset}
          pageSize={pageSize}
          total={total}
          canPrev={offset > 0}
          canNext={offset + pageSize < total}
          onPrev={onPrev}
          onNext={onNext}
        />
      )}
    </div>
  );
}
