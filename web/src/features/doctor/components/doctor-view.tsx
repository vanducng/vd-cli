import type { ColumnDef } from "@tanstack/react-table";

import { DataTable } from "@/features/shared/components/data-table";
import { DriftBadge } from "@/features/shared/components/drift-badge";
import { useDoctorReport } from "../queries";
import type { DoctorEntry } from "../schemas";

const columns: ColumnDef<DoctorEntry>[] = [
  { accessorKey: "skill", header: "Skill" },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ getValue }) => <DriftBadge drift={getValue<string>()} />,
  },
  {
    accessorKey: "detail",
    header: "Detail",
    cell: ({ getValue }) => <span className="font-mono text-xs text-muted-foreground">{getValue<string>()}</span>,
  },
];

export function DoctorView() {
  const { data, isLoading, error } = useDoctorReport();

  return (
    <DataTable
      columns={columns}
      data={data ?? []}
      isLoading={isLoading}
      error={error}
      emptyMessage="No locked skills."
      pageSize={10}
    />
  );
}
