import type { Row } from "./labels";
import { hasDrift, platformLabel } from "./labels";

export function StatBar({ rows }: { rows: Row[] }) {
  const byType = count(rows.map((r) => r.type));
  const byPlat = count(rows.filter((r) => r.scope === "discovered").map((r) => r.platform));
  const drift = rows.filter((r) => r.scope === "managed" && hasDrift(r.drift)).length;
  const managed = rows.filter((r) => r.scope === "managed").length;

  return (
    <div className="statbar">
      <Stat label="Total" value={rows.length} />
      <Stat label="Managed" value={managed} />
      {["skill", "agent", "command", "rule"].map((t) => (
        <Stat key={t} label={t + "s"} value={byType[t] || 0} />
      ))}
      <div className="stat-sep" />
      {Object.entries(byPlat).map(([p, n]) => (
        <Stat key={p} label={platformLabel(p)} value={n} />
      ))}
      {drift > 0 && <Stat label="Drift" value={drift} tone="warn" />}
    </div>
  );
}

function Stat({ label, value, tone }: { label: string; value: number; tone?: string }) {
  return (
    <div className={`stat ${tone ? "stat-" + tone : ""}`}>
      <span className="stat-value">{value}</span>
      <span className="stat-label">{label}</span>
    </div>
  );
}

function count(xs: string[]): Record<string, number> {
  const m: Record<string, number> = {};
  for (const x of xs) m[x] = (m[x] || 0) + 1;
  return m;
}
