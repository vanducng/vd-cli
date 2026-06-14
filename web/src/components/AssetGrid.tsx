import type { Row } from "./labels";
import { typeLabel } from "./labels";
import { DriftBadge } from "./DriftBadge";
import { PlatformBadge } from "./PlatformBadge";

const TYPE_ORDER = ["skill", "agent", "command", "hook", "rule"];

interface Props {
  rows: Row[];
  view: "cards" | "table";
  onOpen: (name: string) => void;
}

export function AssetGrid({ rows, view, onOpen }: Props) {
  if (rows.length === 0) return <p className="muted empty">No assets match the filters.</p>;
  return (
    <>
      {groupByType(rows).map(([type, items]) => (
        <section key={type} className="group">
          <h2>
            {typeLabel(type)} <span className="muted">({items.length})</span>
          </h2>
          {view === "cards" ? <Cards rows={items} onOpen={onOpen} /> : <Table rows={items} onOpen={onOpen} />}
        </section>
      ))}
    </>
  );
}

function Where({ r }: { r: Row }) {
  return r.scope === "managed" ? (
    <span className="badge scope-managed">managed</span>
  ) : (
    <PlatformBadge platform={r.platform} />
  );
}

function Cards({ rows, onOpen }: { rows: Row[]; onOpen: (n: string) => void }) {
  return (
    <div className="cards">
      {rows.map((r) => {
        const open = r.type === "skill" ? () => onOpen(r.name) : undefined;
        return (
          <article key={key(r)} className={`card ${open ? "clickable" : ""}`} onClick={open}>
            <div className="card-head">
              <span className="card-name">{r.name}</span>
              {!r.enabled && <span className="muted"> disabled</span>}
            </div>
            <p className="card-desc">{r.description || <span className="muted">no description</span>}</p>
            <div className="card-foot">
              <Where r={r} />
              {r.source && <span className="muted">{r.source}</span>}
              {r.sha && <span className="mono">{r.sha}</span>}
              <DriftBadge drift={r.drift} />
            </div>
          </article>
        );
      })}
    </div>
  );
}

function Table({ rows, onOpen }: { rows: Row[]; onOpen: (n: string) => void }) {
  return (
    <table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Where</th>
          <th>Drift</th>
          <th>SHA</th>
          <th>Description</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={key(r)}>
            <td>
              {r.type === "skill" ? (
                <button className="link" onClick={() => onOpen(r.name)}>
                  {r.name}
                </button>
              ) : (
                r.name
              )}
              {!r.enabled && <span className="muted"> (disabled)</span>}
            </td>
            <td>
              <Where r={r} />
            </td>
            <td>
              <DriftBadge drift={r.drift} />
            </td>
            <td className="mono">{r.sha}</td>
            <td className="desc">{r.description}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function key(r: Row): string {
  return `${r.scope}/${r.platform}/${r.type}/${r.name}`;
}

function groupByType(rows: Row[]): [string, Row[]][] {
  const m = new Map<string, Row[]>();
  for (const r of rows) {
    const arr = m.get(r.type) ?? [];
    arr.push(r);
    m.set(r.type, arr);
  }
  return [...m.entries()].sort((a, b) => TYPE_ORDER.indexOf(a[0]) - TYPE_ORDER.indexOf(b[0]));
}
