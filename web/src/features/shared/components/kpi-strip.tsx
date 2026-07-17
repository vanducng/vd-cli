import { cn } from "@/lib/utils";

export interface Kpi {
  label: string;
  value: string | number;
  sublabel?: string;
  tone?: "default" | "accent" | "warn";
}

/** The winning mock's metric strip: label-over-value cells in one bordered row,
 * warn tone reserved for drift-style attention counts. */
export function KpiStrip({ items, className }: { items: Kpi[]; className?: string }) {
  return (
    <div
      className={cn(
        "mb-4 grid grid-cols-2 gap-px overflow-hidden rounded-md border border-border bg-border sm:grid-cols-3 md:grid-cols-[repeat(auto-fit,minmax(140px,1fr))]",
        className,
      )}
    >
      {items.map((k) => (
        <div key={k.label} className="bg-panel px-4 py-3">
          <div className="text-xs uppercase tracking-wide text-faint">{k.label}</div>
          <div
            className={cn(
              "mt-1 text-2xl font-semibold tabular-nums leading-tight",
              k.tone === "accent" && "text-primary",
              k.tone === "warn" && "text-warn",
            )}
          >
            {k.value}
          </div>
          {k.sublabel && <div className="mt-0.5 text-xs text-faint">{k.sublabel}</div>}
        </div>
      ))}
    </div>
  );
}
