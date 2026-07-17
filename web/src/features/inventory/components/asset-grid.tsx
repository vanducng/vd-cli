import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { DriftBadge } from "@/features/shared/components/drift-badge";
import { PlatformBadge } from "./platform-badge";
import { typeLabel, type Row } from "./labels";

const TYPE_ORDER = ["skill", "agent", "command", "hook", "rule"];

interface AssetGridProps {
  rows: Row[];
  view: "cards" | "table";
  onOpen: (name: string) => void;
}

export function AssetGrid({ rows, view, onOpen }: AssetGridProps) {
  if (rows.length === 0) {
    return <p className="py-10 text-center text-sm text-muted-foreground">No assets match the filters.</p>;
  }
  return (
    <div className="flex flex-col gap-6">
      {groupByType(rows).map(([type, items]) => (
        <section key={type}>
          <h2 className="mb-2 text-sm font-semibold text-foreground">
            {typeLabel(type)} <span className="font-normal text-muted-foreground">({items.length})</span>
          </h2>
          {view === "cards" ? <CardsView rows={items} onOpen={onOpen} /> : <TableView rows={items} onOpen={onOpen} />}
        </section>
      ))}
    </div>
  );
}

function Where({ r }: { r: Row }) {
  return r.scope === "managed" ? <Badge variant="amber">managed</Badge> : <PlatformBadge platform={r.platform} />;
}

function CardsView({ rows, onOpen }: { rows: Row[]; onOpen: (n: string) => void }) {
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-3">
      {rows.map((r) => {
        const openable = r.type === "skill";
        return (
          <Card
            key={rowKey(r)}
            className={cn("flex flex-col gap-2", openable && "cursor-pointer hover:border-primary/50")}
            onClick={openable ? () => onOpen(r.name) : undefined}
            role={openable ? "button" : undefined}
            tabIndex={openable ? 0 : undefined}
            onKeyDown={
              openable
                ? (e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      onOpen(r.name);
                    }
                  }
                : undefined
            }
          >
            <div className="flex items-center justify-between gap-2">
              <span className="truncate font-medium">{r.name}</span>
              {!r.enabled && <span className="text-xs text-muted-foreground">disabled</span>}
            </div>
            <p className="line-clamp-2 text-sm text-muted-foreground">
              {r.description || <span className="text-faint">no description</span>}
            </p>
            <div className="mt-1 flex flex-wrap items-center gap-2 text-xs">
              <Where r={r} />
              {r.source && <span className="text-muted-foreground">{r.source}</span>}
              {r.sha && <span className="font-mono text-faint">{r.sha}</span>}
              <DriftBadge drift={r.drift} />
            </div>
          </Card>
        );
      })}
    </div>
  );
}

function TableView({ rows, onOpen }: { rows: Row[]; onOpen: (n: string) => void }) {
  return (
    <div className="rounded-md border border-border">
      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead>Name</TableHead>
            <TableHead>Where</TableHead>
            <TableHead>Drift</TableHead>
            <TableHead>SHA</TableHead>
            <TableHead>Description</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((r) => (
            <TableRow key={rowKey(r)}>
              <TableCell>
                {r.type === "skill" ? (
                  <button type="button" className="text-info hover:underline" onClick={() => onOpen(r.name)}>
                    {r.name}
                  </button>
                ) : (
                  r.name
                )}
                {!r.enabled && <span className="text-muted-foreground"> (disabled)</span>}
              </TableCell>
              <TableCell>
                <Where r={r} />
              </TableCell>
              <TableCell>
                <DriftBadge drift={r.drift} />
              </TableCell>
              <TableCell className="font-mono text-xs text-faint">{r.sha}</TableCell>
              <TableCell className="max-w-[360px] truncate text-muted-foreground">{r.description}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function rowKey(r: Row): string {
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
