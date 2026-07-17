import { Table, TableBody, TableCell, TableFooter, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import { AgentBadge } from "@/features/obs/components/agent-badge";
import { CostCell } from "@/features/obs/components/cost-cell";
import { TokenCell } from "@/features/obs/components/token-cell";
import type { TokenUsage, UsageRow } from "@/features/obs/schemas";

function CountCell({ value }: { value: number }) {
  if (value === 0) return <span className="text-muted-foreground">—</span>;
  return <TokenCell tokens={value} />;
}

interface UsageTableProps {
  rows: UsageRow[];
  totals: TokenUsage;
  totalCostUsd: number | null;
  isLoading?: boolean;
  error?: Error | null;
}

const COLS = 8;

/** Per-model breakdown beneath the usage chart, mirroring `vd obs usage`'s
 * DATE AGENT MODEL INPUT OUTPUT CACHE R CACHE W EST $ table. */
export function UsageTable({ rows, totals, totalCostUsd, isLoading, error }: UsageTableProps) {
  return (
    <div className="overflow-x-auto rounded-md border border-border bg-panel">
      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead>Date</TableHead>
            <TableHead>Agent</TableHead>
            <TableHead>Model</TableHead>
            <TableHead className="text-right">Input</TableHead>
            <TableHead className="text-right">Output</TableHead>
            <TableHead className="text-right">Cache R</TableHead>
            <TableHead className="text-right">Cache W</TableHead>
            <TableHead className="text-right">Est $</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {error ? (
            <TableRow>
              <TableCell colSpan={COLS} className="h-24 text-center text-err">
                {error.message}
              </TableCell>
            </TableRow>
          ) : isLoading ? (
            Array.from({ length: 4 }).map((_, i) => (
              <TableRow key={i} className="hover:bg-transparent">
                {Array.from({ length: COLS }).map((_, j) => (
                  <TableCell key={j}>
                    <Skeleton className="h-4 w-full max-w-[120px]" />
                  </TableCell>
                ))}
              </TableRow>
            ))
          ) : rows.length === 0 ? (
            <TableRow className="hover:bg-transparent">
              <TableCell colSpan={COLS} className="h-24 text-center text-muted-foreground">
                No usage in this range yet.
              </TableCell>
            </TableRow>
          ) : (
            rows.map((row, i) => (
              <TableRow key={`${row.date}-${row.agent}-${row.model}-${i}`}>
                <TableCell className="font-mono text-muted-foreground">
                  {i > 0 && rows[i - 1].date === row.date ? "" : row.date}
                </TableCell>
                <TableCell>
                  <AgentBadge agent={row.agent} />
                </TableCell>
                <TableCell className="font-mono">{row.model || "(unknown)"}</TableCell>
                <TableCell className="text-right">
                  <CountCell value={row.tokens.input} />
                </TableCell>
                <TableCell className="text-right">
                  <CountCell value={row.tokens.output} />
                </TableCell>
                <TableCell className="text-right">
                  <CountCell value={row.tokens.cacheread} />
                </TableCell>
                <TableCell className="text-right">
                  <CountCell value={row.tokens.cachewrite} />
                </TableCell>
                <TableCell className="text-right">
                  <CostCell costUsd={row.costusd} model={row.model} />
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
        {!isLoading && !error && rows.length > 0 && (
          <TableFooter>
            <TableRow className="hover:bg-transparent">
              <TableCell colSpan={3} className="font-semibold">
                TOTAL
              </TableCell>
              <TableCell className="text-right font-semibold">
                <TokenCell tokens={totals.input} />
              </TableCell>
              <TableCell className="text-right font-semibold">
                <TokenCell tokens={totals.output} />
              </TableCell>
              <TableCell className="text-right font-semibold">
                <TokenCell tokens={totals.cacheread} />
              </TableCell>
              <TableCell className="text-right font-semibold">
                <TokenCell tokens={totals.cachewrite} />
              </TableCell>
              <TableCell className="text-right font-semibold">
                <CostCell costUsd={totalCostUsd} />
              </TableCell>
            </TableRow>
          </TableFooter>
        )}
      </Table>
    </div>
  );
}
