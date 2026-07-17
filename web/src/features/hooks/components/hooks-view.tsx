import type { ColumnDef } from "@tanstack/react-table";

import { Badge } from "@/components/ui/badge";
import { DataTable } from "@/features/shared/components/data-table";
import { KpiStrip } from "@/features/shared/components/kpi-strip";
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
  const hooks = data ?? [];
  const managed = hooks.filter((h) => h.frontmatter?.managedByVd).length;

  return (
    <div>
      {!isLoading && !error && (
        <KpiStrip
          items={[
            { label: "Total", value: hooks.length, sublabel: "registered hooks" },
            { label: "vd-managed", value: managed, tone: "accent" },
          ]}
        />
      )}
      <DataTable
        columns={columns}
        data={hooks}
        isLoading={isLoading}
        error={error}
        emptyMessage="No hooks registered."
        pageSize={10}
      />
    </div>
  );
}
