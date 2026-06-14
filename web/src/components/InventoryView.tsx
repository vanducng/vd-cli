import { useMemo, useState } from "react";
import type { Inventory } from "../types";
import type { Row } from "./labels";
import { hasDrift } from "./labels";
import { StatBar } from "./StatBar";
import { FilterBar } from "./FilterBar";
import { AssetGrid } from "./AssetGrid";

interface Props {
  inv: Inventory | null;
  onOpen: (name: string) => void;
}

export function InventoryView({ inv, onOpen }: Props) {
  const [type, setType] = useState("all");
  const [platform, setPlatform] = useState("all");
  const [scope, setScope] = useState("all");
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState("name");
  const [view, setView] = useState<"cards" | "table">("cards");

  const all: Row[] = useMemo(
    () =>
      inv
        ? [
            ...inv.managed.map((a) => ({ ...a, scope: "managed" as const })),
            ...inv.discovered.map((a) => ({ ...a, scope: "discovered" as const })),
          ]
        : [],
    [inv],
  );

  const rows = useMemo(() => {
    let r = all;
    if (type !== "all") r = r.filter((x) => x.type === type);
    if (platform !== "all") r = r.filter((x) => x.platform === platform);
    if (scope !== "all") r = r.filter((x) => x.scope === scope);
    if (query) {
      const q = query.toLowerCase();
      r = r.filter((x) => `${x.name} ${x.description} ${x.source ?? ""}`.toLowerCase().includes(q));
    }
    return sortRows(r, sort);
  }, [all, type, platform, scope, query, sort]);

  if (!inv) return <p>Loading…</p>;

  return (
    <div>
      <StatBar rows={all} />
      <FilterBar
        type={type}
        setType={setType}
        platform={platform}
        setPlatform={setPlatform}
        scope={scope}
        setScope={setScope}
        query={query}
        setQuery={setQuery}
        sort={sort}
        setSort={setSort}
        view={view}
        setView={setView}
      />
      <p className="muted result-count">
        {rows.length} of {all.length}
      </p>
      <AssetGrid rows={rows} view={view} onOpen={onOpen} />
    </div>
  );
}

function sortRows(r: Row[], sort: string): Row[] {
  const s = [...r];
  s.sort((a, b) => {
    if (sort === "type" && a.type !== b.type) return a.type < b.type ? -1 : 1;
    if (sort === "platform" && a.platform !== b.platform) return a.platform < b.platform ? -1 : 1;
    if (sort === "drift") {
      const da = hasDrift(a.drift) ? 0 : 1;
      const db = hasDrift(b.drift) ? 0 : 1;
      if (da !== db) return da - db;
    }
    return a.name.localeCompare(b.name);
  });
  return s;
}
