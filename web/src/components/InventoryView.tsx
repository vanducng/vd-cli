import { useState } from "react";
import type { Inventory, AssetSummary } from "../types";
import { DriftBadge } from "./DriftBadge";

interface Props {
  inv: Inventory | null;
  onOpen: (name: string) => void;
}

export function InventoryView({ inv, onOpen }: Props) {
  const [q, setQ] = useState("");
  if (!inv) return <p>Loading…</p>;

  const match = (a: AssetSummary) =>
    !q || `${a.name} ${a.description} ${a.source ?? ""} ${a.type}`.toLowerCase().includes(q.toLowerCase());

  return (
    <div>
      <input
        className="search"
        placeholder="Search name, description, source…"
        value={q}
        onChange={(e) => setQ(e.target.value)}
      />
      <Section title="Managed" rows={inv.managed.filter(match)} total={inv.managed.length} onOpen={onOpen} />
      <Section title="Discovered" rows={inv.discovered.filter(match)} total={inv.discovered.length} onOpen={onOpen} />
    </div>
  );
}

function Section({
  title,
  rows,
  total,
  onOpen,
}: {
  title: string;
  rows: AssetSummary[];
  total: number;
  onOpen: (n: string) => void;
}) {
  return (
    <section>
      <h2>
        {title} <span className="muted">({rows.length}/{total})</span>
      </h2>
      <table>
        <thead>
          <tr>
            <th>Type</th>
            <th>Name</th>
            <th>Description</th>
            <th>Source</th>
            <th>Mode</th>
            <th>SHA</th>
            <th>Drift</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((a) => (
            <tr key={`${a.type}/${a.name}`}>
              <td>{a.type}</td>
              <td>
                {a.type === "skill" ? (
                  <button className="link" onClick={() => onOpen(a.name)}>
                    {a.name}
                  </button>
                ) : (
                  a.name
                )}
                {!a.enabled && <span className="muted"> (disabled)</span>}
              </td>
              <td className="desc">{a.description}</td>
              <td>{a.source}</td>
              <td>{a.mode}</td>
              <td className="mono">{a.sha}</td>
              <td>
                <DriftBadge drift={a.drift} />
              </td>
            </tr>
          ))}
          {rows.length === 0 && (
            <tr>
              <td colSpan={7} className="muted">
                none
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </section>
  );
}
