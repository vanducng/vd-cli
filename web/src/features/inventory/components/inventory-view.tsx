import { useMemo, useState } from "react";

import { Skeleton } from "@/components/ui/skeleton";
import { useInventory } from "../queries";
import { AssetGrid } from "./asset-grid";
import { FilterBar } from "./filter-bar";
import { StatBar } from "./stat-bar";
import { hasDrift, type Row } from "./labels";

interface InventoryViewProps {
  onOpen: (name: string) => void;
}

export function InventoryView({ onOpen }: InventoryViewProps) {
  const { data: inv, isLoading, error } = useInventory();
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

  if (error) {
    return <p className="text-sm text-err">{error.message}</p>;
  }

  if (isLoading) {
    return (
      <div className="flex flex-col gap-4">
        <div className="grid grid-cols-[repeat(auto-fit,minmax(110px,1fr))] gap-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-16" />
          ))}
        </div>
        <Skeleton className="h-9" />
        <Skeleton className="h-64" />
      </div>
    );
  }

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
      <p className="mb-3 text-sm text-muted-foreground">
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
