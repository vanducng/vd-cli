import { cn } from "@/lib/utils";

function humanize(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`;
  return String(n);
}

interface TokenCellProps {
  tokens: number;
  className?: string;
}

/** Compact humanized token count (412.3k / 1.2M); exact value on hover. */
export function TokenCell({ tokens, className }: TokenCellProps) {
  return (
    <span className={cn("tabular-nums", className)} title={tokens.toLocaleString()}>
      {humanize(tokens)}
    </span>
  );
}
