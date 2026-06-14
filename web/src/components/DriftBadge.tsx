export function DriftBadge({ drift }: { drift?: string }) {
  if (!drift) return null;
  return <span className={`badge drift-${drift}`}>{drift}</span>;
}
