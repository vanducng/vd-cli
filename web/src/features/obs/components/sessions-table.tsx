import { useMemo, type ComponentType, type ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";

import { DataTable } from "@/features/shared/components/data-table";
import { AgentBadge } from "@/features/obs/components/agent-badge";
import { CostCell } from "@/features/obs/components/cost-cell";
import { TokenCell } from "@/features/obs/components/token-cell";
import type { SessionSummary } from "@/features/obs/schemas";

// /obs/sessions/$id lands in phase 5; TanStack's generated route registry can't
// type-check a Link to it yet. Widen just this one usage rather than casting at
// every call site — resolves to a real typed route once phase 5 adds the file.
const SessionLink = Link as unknown as ComponentType<{
  to: "/obs/sessions/$id";
  params: { id: string };
  className?: string;
  children?: ReactNode;
}>;

function formatStarted(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function totalTokens(s: SessionSummary): number {
  return s.tokens.input + s.tokens.output + s.tokens.cacheread + s.tokens.cachewrite;
}

interface SessionsTableProps {
  sessions: SessionSummary[];
  isLoading?: boolean;
  error?: Error | null;
  isFiltered?: boolean;
  onClearFilters?: () => void;
  toolbar?: React.ReactNode;
  pageSize?: number;
}

export function SessionsTable({
  sessions,
  isLoading,
  error,
  isFiltered,
  onClearFilters,
  toolbar,
  pageSize,
}: SessionsTableProps) {
  const columns = useMemo<ColumnDef<SessionSummary>[]>(
    () => [
      {
        accessorKey: "startedat",
        header: "Started",
        cell: ({ row }) => (
          <span className="font-mono text-muted-foreground">{formatStarted(row.original.startedat)}</span>
        ),
      },
      {
        accessorKey: "agent",
        header: "Agent",
        cell: ({ row }) => <AgentBadge agent={row.original.agent} />,
      },
      {
        accessorKey: "title",
        header: "Title",
        cell: ({ row }) => {
          const { id, title, agent } = row.original;
          if (!title) {
            const kind = agent === "codex" ? "codex" : agent;
            return <span className="text-faint">— no title ({kind})</span>;
          }
          return (
            <SessionLink to="/obs/sessions/$id" params={{ id }} className="text-info hover:underline">
              {title}
            </SessionLink>
          );
        },
      },
      {
        accessorKey: "model",
        header: "Model",
        cell: ({ row }) => <span className="font-mono text-muted-foreground">{row.original.model}</span>,
      },
      {
        accessorKey: "turncount",
        header: () => <div className="text-right">Turns</div>,
        cell: ({ row }) => <div className="text-right tabular-nums">{row.original.turncount}</div>,
      },
      {
        id: "tokens",
        header: () => <div className="text-right">Tokens</div>,
        cell: ({ row }) => (
          <div className="text-right">
            <TokenCell tokens={totalTokens(row.original)} />
          </div>
        ),
      },
      {
        id: "costusd",
        header: () => <div className="text-right">Est $</div>,
        cell: ({ row }) => (
          <div className="text-right">
            <CostCell costUsd={row.original.costusd} model={row.original.model} />
          </div>
        ),
      },
    ],
    [],
  );

  return (
    <DataTable
      columns={columns}
      data={sessions}
      isLoading={isLoading}
      error={error}
      toolbar={toolbar}
      isFiltered={isFiltered}
      onClearFilters={onClearFilters}
      emptyMessage="No sessions synced yet."
      filteredEmptyMessage="No sessions match the current filters."
      pageSize={pageSize}
    />
  );
}
