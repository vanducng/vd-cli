import { cn } from "@/lib/utils";

interface CostCellProps {
  costUsd: number | null;
  model?: string;
  className?: string;
}

/** Renders an est-$ value. Unpriced (null) cost renders "?" with a tooltip naming
 * the model and where to add a price, never "$0.00" (a zero reads as free). */
export function CostCell({ costUsd, model, className }: CostCellProps) {
  if (costUsd === null) {
    const title = model
      ? `${model} has no price entry, add one to ~/.vd/obs/prices.json`
      : "No price entry, add one to ~/.vd/obs/prices.json";
    return (
      <span className={cn("font-mono text-muted-foreground", className)} title={title}>
        ?
      </span>
    );
  }
  return <span className={cn("tabular-nums", className)}>{costUsd.toFixed(2)}</span>;
}
