import { type ReactNode } from "react";
import {
  type ColumnDef,
  flexRender,
  getCoreRowModel,
  getPaginationRowModel,
  useReactTable,
} from "@tanstack/react-table";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";

interface DataTableProps<TData, TValue> {
  columns: ColumnDef<TData, TValue>[];
  data: TData[];
  isLoading?: boolean;
  error?: Error | null;
  toolbar?: ReactNode;
  isFiltered?: boolean;
  onClearFilters?: () => void;
  emptyMessage?: string;
  filteredEmptyMessage?: string;
  pageSize?: number;
  className?: string;
}

/** One TanStack Table shell shared by every list view: loading, empty, filtered-empty,
 * and error states, plus a mobile-friendly toolbar slot and pinned pagination footer. */
export function DataTable<TData, TValue>({
  columns,
  data,
  isLoading = false,
  error = null,
  toolbar,
  isFiltered = false,
  onClearFilters,
  emptyMessage = "No data yet.",
  filteredEmptyMessage = "No rows match the current filters.",
  pageSize = 10,
  className,
}: DataTableProps<TData, TValue>) {
  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: { pagination: { pageSize } },
  });

  const rows = table.getRowModel().rows;
  const showSkeleton = isLoading && rows.length === 0;

  return (
    <div className={cn("flex flex-col gap-3", className)}>
      {toolbar && <div className="flex flex-wrap gap-2">{toolbar}</div>}

      <div className="max-h-[75vh] overflow-auto rounded-md border border-border">
        <Table>
          <TableHeader className="sticky top-0 z-10 bg-panel">
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className="hover:bg-transparent">
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id}>
                    {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody className="min-h-[420px]">
            {error ? (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-[420px] text-center text-err">
                  {error.message}
                </TableCell>
              </TableRow>
            ) : showSkeleton ? (
              Array.from({ length: pageSize }).map((_, i) => (
                <TableRow key={i} className="hover:bg-transparent">
                  {columns.map((_, j) => (
                    <TableCell key={j}>
                      <Skeleton className="h-4 w-full max-w-[160px]" />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : rows.length === 0 ? (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={columns.length} className="h-[420px] text-center align-middle">
                  <div className="flex h-full flex-col items-center justify-center gap-2 text-muted-foreground">
                    <p className="text-sm">{isFiltered ? filteredEmptyMessage : emptyMessage}</p>
                    {isFiltered && onClearFilters && (
                      <Button variant="outline" size="sm" onClick={onClearFilters}>
                        Clear filters
                      </Button>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            ) : (
              rows.map((row) => (
                <TableRow key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</TableCell>
                  ))}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <PaginationBar
        offset={table.getState().pagination.pageIndex * table.getState().pagination.pageSize}
        pageSize={table.getState().pagination.pageSize}
        total={data.length}
        canPrev={table.getCanPreviousPage()}
        canNext={table.getCanNextPage()}
        onPrev={() => table.previousPage()}
        onNext={() => table.nextPage()}
        onPageSize={(n) => table.setPageSize(n)}
      />
    </div>
  );
}

interface PaginationBarProps {
  offset: number;
  pageSize: number;
  total: number;
  canPrev: boolean;
  canNext: boolean;
  onPrev: () => void;
  onNext: () => void;
  onPageSize?: (n: number) => void;
}

/** Rows selector · "1–25 of 174" · Prev/Next.
 * Shared by client-paged DataTable and the server-paged sessions view. */
export function PaginationBar({ offset, pageSize, total, canPrev, canNext, onPrev, onNext, onPageSize }: PaginationBarProps) {
  const start = total === 0 ? 0 : offset + 1;
  const end = Math.min(offset + pageSize, total);

  return (
    <div className="flex items-center justify-between gap-3 pt-1 text-sm text-muted-foreground">
      <div className="flex items-center gap-2">
        {onPageSize && (
          <>
            <span className="text-xs uppercase tracking-wide text-faint">Rows</span>
            <select
              className="h-8 rounded-sm border border-border bg-panel px-2 text-sm text-foreground"
              value={pageSize}
              onChange={(e) => onPageSize(Number(e.target.value))}
              aria-label="Rows per page"
            >
              {[10, 25, 50, 100].map((n) => (
                <option key={n} value={n}>
                  {n}
                </option>
              ))}
            </select>
          </>
        )}
        <span className="tabular-nums">
          {start}–{end} of {total}
        </span>
      </div>
      <div className="flex gap-2">
        <Button variant="outline" size="sm" onClick={onPrev} disabled={!canPrev}>
          Prev
        </Button>
        <Button variant="outline" size="sm" onClick={onNext} disabled={!canNext}>
          Next
        </Button>
      </div>
    </div>
  );
}
