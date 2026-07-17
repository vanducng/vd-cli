import { Badge, type BadgeProps } from "@/components/ui/badge";

// Shared by inventory (asset-grid, skill-detail-view) and doctor (status column).
const VARIANT: Record<string, NonNullable<BadgeProps["variant"]>> = {
  none: "ok",
  local: "amber",
  missing: "err",
  unknown: "err",
  untracked: "default",
};

export function DriftBadge({ drift }: { drift?: string }) {
  if (!drift) return null;
  return <Badge variant={VARIANT[drift] ?? "default"}>{drift}</Badge>;
}
