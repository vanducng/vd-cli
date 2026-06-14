import type { ReactNode } from "react";
import { TYPES, PLATFORMS, SCOPES, typeLabel, platformLabel, cap } from "./labels";

interface Props {
  type: string;
  setType: (v: string) => void;
  platform: string;
  setPlatform: (v: string) => void;
  scope: string;
  setScope: (v: string) => void;
  query: string;
  setQuery: (v: string) => void;
  sort: string;
  setSort: (v: string) => void;
  view: "cards" | "table";
  setView: (v: "cards" | "table") => void;
}

export function FilterBar(p: Props) {
  return (
    <div className="filterbar">
      <input
        className="search"
        placeholder="Search name, description, source…"
        value={p.query}
        onChange={(e) => p.setQuery(e.target.value)}
      />
      <div className="filter-row">
        <span className="filter-label">Type</span>
        {TYPES.map((t) => (
          <Chip key={t} on={p.type === t} onClick={() => p.setType(t)}>
            {typeLabel(t)}
          </Chip>
        ))}
      </div>
      <div className="filter-row">
        <span className="filter-label">Agent</span>
        {PLATFORMS.map((pl) => (
          <Chip key={pl} on={p.platform === pl} onClick={() => p.setPlatform(pl)}>
            {pl === "all" ? "All" : platformLabel(pl)}
          </Chip>
        ))}
      </div>
      <div className="filter-row">
        <span className="filter-label">Scope</span>
        {SCOPES.map((s) => (
          <Chip key={s} on={p.scope === s} onClick={() => p.setScope(s)}>
            {cap(s)}
          </Chip>
        ))}
        <span className="row-end">
          <label className="sortlabel">
            Sort
            <select value={p.sort} onChange={(e) => p.setSort(e.target.value)}>
              <option value="name">Name</option>
              <option value="type">Type</option>
              <option value="platform">Agent</option>
              <option value="drift">Drift first</option>
            </select>
          </label>
          <span className="viewtoggle">
            <button className={p.view === "cards" ? "active" : ""} onClick={() => p.setView("cards")}>
              Cards
            </button>
            <button className={p.view === "table" ? "active" : ""} onClick={() => p.setView("table")}>
              Table
            </button>
          </span>
        </span>
      </div>
    </div>
  );
}

function Chip({ on, onClick, children }: { on: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button className={`chip ${on ? "chip-on" : ""}`} onClick={onClick}>
      {children}
    </button>
  );
}
