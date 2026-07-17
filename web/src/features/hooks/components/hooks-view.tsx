import type { ColumnDef } from "@tanstack/react-table";

import { Badge } from "@/components/ui/badge";
import { DataTable } from "@/features/shared/components/data-table";
import { useHooks } from "../queries";
import type { HookAsset } from "../schemas";

const columns: ColumnDef<HookAsset>[] = [
  { accessorKey: "name", header: "Event" },
  {
    accessorKey: "description",
    header: "Command",
    cell: ({ getValue }) => <span className="font-mono text-xs">{getValue<string>()}</span>,
  },
  {
    id: "managedByVd",
    header: "vd",
    cell: ({ row }) => (row.original.frontmatter?.managedByVd ? <Badge variant="ok">vd</Badge> : null),
  },
];

export function HooksView() {
  const { data, isLoading, error } = useHooks();

  return (
    <DataTable
      columns={columns}
      data={data ?? []}
      isLoading={isLoading}
      error={error}
      emptyMessage="No hooks registered."
      pageSize={10}
    />
  );
}
