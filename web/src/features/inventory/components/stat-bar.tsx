import { cn } from "@/lib/utils";
import { hasDrift, platformLabel, type Row } from "./labels";

export function StatBar({ rows }: { rows: Row[] }) {
  const byType = count(rows.map((r) => r.type));
  const byPlatform = count(rows.filter((r) => r.scope === "discovered").map((r) => r.platform));
  const managed = rows.filter((r) => r.scope === "managed").length;
  const drift = rows.filter((r) => r.scope === "managed" && hasDrift(r.drift)).length;

  return (
    <div className="mb-4 grid grid-cols-[repeat(auto-fit,minmax(110px,1fr))] gap-3">
      <Stat label="Total" value={rows.length} />
      <Stat label="Managed" value={managed} />
      {(["skill", "agent", "command", "rule"] as const).map((t) => (
        <Stat key={t} label={`${t}s`} value={byType[t] ?? 0} />
      ))}
      {Object.entries(byPlatform).map(([p, n]) => (
        <Stat key={p} label={platformLabel(p)} value={n} />
      ))}
      {drift > 0 && <Stat label="Drift" value={drift} tone="warn" />}
    </div>
  );
}

function Stat({ label, value, tone }: { label: string; value: number; tone?: "warn" }) {
  return (
    <div className="rounded-md border border-border bg-panel px-4 py-3">
      <b className={cn("block text-xl tabular-nums", tone === "warn" && "text-primary")}>{value}</b>
      <span className="text-xs uppercase tracking-wide text-faint">{label}</span>
    </div>
  );
}

function count(xs: string[]): Record<string, number> {
  const m: Record<string, number> = {};
  for (const x of xs) m[x] = (m[x] ?? 0) + 1;
  return m;
}
