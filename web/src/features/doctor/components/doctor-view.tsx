import type { ColumnDef } from "@tanstack/react-table";

import { DataTable } from "@/features/shared/components/data-table";
import { DriftBadge } from "@/features/shared/components/drift-badge";
import { KpiStrip } from "@/features/shared/components/kpi-strip";
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
  const entries = data ?? [];
  const drift = entries.filter((e) => e.status && e.status !== "none").length;

  return (
    <div>
      {!isLoading && !error && (
        <KpiStrip
          items={[
            { label: "Total", value: entries.length, sublabel: "locked skills" },
            ...(drift > 0 ? [{ label: "Drift", value: drift, tone: "warn" as const }] : []),
          ]}
        />
      )}
      <DataTable
        columns={columns}
        data={entries}
        isLoading={isLoading}
        error={error}
        emptyMessage="No locked skills."
        pageSize={10}
      />
    </div>
  );
}
