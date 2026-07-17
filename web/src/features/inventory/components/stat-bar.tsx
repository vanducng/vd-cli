import { KpiStrip, type Kpi } from "@/features/shared/components/kpi-strip";
import type { GroupedAsset } from "../group-assets";
import { hasDrift, platformLabel } from "./labels";

const TYPE_ORDER = ["skill", "agent", "command", "rule"] as const;

export function StatBar({ assets }: { assets: GroupedAsset[] }) {
  const byType = count(assets.map((a) => a.type));
  const managed = assets.filter((a) => a.managed).length;
  const claude = assets.filter((a) => a.platforms.includes("claude_code")).length;
  const codex = assets.filter((a) => a.platforms.includes("codex")).length;
  const drift = assets.filter((a) => hasDrift(a.drift)).length;

  const items: Kpi[] = [
    { label: "Total", value: assets.length, sublabel: "tracked assets" },
    { label: "Managed", value: managed, tone: "accent" },
    ...TYPE_ORDER.map((t) => ({ label: `${t}s`, value: byType[t] ?? 0 })),
    { label: platformLabel("claude_code"), value: claude },
    { label: platformLabel("codex"), value: codex },
  ];
  if (drift > 0) items.push({ label: "Drift", value: drift, tone: "warn" });

  return <KpiStrip items={items} />;
}

function count(xs: string[]): Record<string, number> {
  const m: Record<string, number> = {};
  for (const x of xs) m[x] = (m[x] ?? 0) + 1;
  return m;
}
