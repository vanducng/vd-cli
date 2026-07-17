import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { DriftBadge } from "@/features/shared/components/drift-badge";
import { cn } from "@/lib/utils";
import type { GroupedAsset } from "../group-assets";
import { PlatformBadgeRow } from "./platform-badge";
import { hasDrift, typeLabel } from "./labels";

const TYPE_ORDER = ["skill", "agent", "command", "hook", "rule"];

interface AssetGridProps {
  assets: GroupedAsset[];
  view: "cards" | "table";
  onOpen: (name: string) => void;
}

export function AssetGrid({ assets, view, onOpen }: AssetGridProps) {
  if (assets.length === 0) {
    return <p className="py-10 text-center text-sm text-muted-foreground">No assets match the filters.</p>;
  }
  return (
    <div className="flex flex-col gap-6">
      {groupByType(assets).map(([type, items]) => (
        <section key={type}>
          <h2 className="mb-2 border-b border-border pb-1.5 text-xs font-semibold uppercase tracking-wide text-faint">
            {typeLabel(type)} <span className="normal-case text-faint">({items.length})</span>
          </h2>
          {view === "cards" ? <CardsView assets={items} onOpen={onOpen} /> : <TableView assets={items} onOpen={onOpen} />}
        </section>
      ))}
    </div>
  );
}

function TypeChip({ type }: { type: string }) {
  return (
    <span className="shrink-0 rounded-sm border border-border px-1.5 py-0.5 font-mono text-[10px] font-bold uppercase tracking-wide text-faint">
      {type}
    </span>
  );
}

function DriftPill() {
  return <Badge className="ml-auto shrink-0 border-warn/50 text-warn">drift</Badge>;
}

function CardsView({ assets, onOpen }: { assets: GroupedAsset[]; onOpen: (n: string) => void }) {
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-3">
      {assets.map((a) => {
        const openable = a.type === "skill";
        const drift = hasDrift(a.drift);
        return (
          <Card
            key={assetKey(a)}
            className={cn("flex flex-col gap-3", openable && "cursor-pointer hover:border-primary/50")}
            onClick={openable ? () => onOpen(a.name) : undefined}
            role={openable ? "button" : undefined}
            tabIndex={openable ? 0 : undefined}
            onKeyDown={
              openable
                ? (e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      onOpen(a.name);
                    }
                  }
                : undefined
            }
          >
            <div className="flex items-center gap-2">
              <TypeChip type={a.type} />
              <span className="truncate font-medium">{a.name}</span>
              {drift && <DriftPill />}
              {!drift && !a.enabled && <span className="ml-auto shrink-0 text-xs text-muted-foreground">disabled</span>}
            </div>
            <p className="line-clamp-2 text-sm text-muted-foreground">
              {a.description || <span className="text-faint">no description</span>}
            </p>
            <PlatformBadgeRow platforms={a.platforms} />
            {(a.managed || a.source || a.sha) && (
              <div className="flex items-center gap-2 border-t border-dashed border-border pt-2 font-mono text-xs text-faint">
                {a.managed && <Badge variant="amber">managed</Badge>}
                {a.source && <span className="truncate text-muted-foreground">{a.source}</span>}
                {a.sha && <span className="ml-auto shrink-0 rounded-sm bg-panel-2 px-1.5 py-0.5 text-faint">{a.sha}</span>}
              </div>
            )}
          </Card>
        );
      })}
    </div>
  );
}

function TableView({ assets, onOpen }: { assets: GroupedAsset[]; onOpen: (n: string) => void }) {
  return (
    <div className="rounded-md border border-border">
      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead>Name</TableHead>
            <TableHead>Platforms</TableHead>
            <TableHead>Managed</TableHead>
            <TableHead>Drift</TableHead>
            <TableHead>SHA</TableHead>
            <TableHead>Description</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {assets.map((a) => (
            <TableRow key={assetKey(a)}>
              <TableCell>
                {a.type === "skill" ? (
                  <button type="button" className="text-info hover:underline" onClick={() => onOpen(a.name)}>
                    {a.name}
                  </button>
                ) : (
                  a.name
                )}
                {!a.enabled && <span className="text-muted-foreground"> (disabled)</span>}
              </TableCell>
              <TableCell>
                <PlatformBadgeRow platforms={a.platforms} />
              </TableCell>
              <TableCell>{a.managed && <Badge variant="amber">managed</Badge>}</TableCell>
              <TableCell>
                <DriftBadge drift={a.drift} />
              </TableCell>
              <TableCell className="font-mono text-xs text-faint">{a.sha}</TableCell>
              <TableCell className="max-w-[360px] truncate text-muted-foreground">{a.description}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function assetKey(a: GroupedAsset): string {
  return `${a.type}/${a.name}`;
}

function groupByType(assets: GroupedAsset[]): [string, GroupedAsset[]][] {
  const m = new Map<string, GroupedAsset[]>();
  for (const a of assets) {
    const arr = m.get(a.type) ?? [];
    arr.push(a);
    m.set(a.type, arr);
  }
  return [...m.entries()].sort((a, b) => TYPE_ORDER.indexOf(a[0]) - TYPE_ORDER.indexOf(b[0]));
}
