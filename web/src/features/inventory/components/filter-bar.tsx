import type { ChangeEvent } from "react";
import { LayoutGrid, List } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { cap, platformLabel, typeLabel, PLATFORMS, SCOPES, TYPES } from "./labels";

interface FilterBarProps {
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

// Compact toolbar (search + selects); fastreact rule 8 forbids the old flat pill row.
export function FilterBar(props: FilterBarProps) {
  return (
    <div className="mb-4 flex flex-wrap gap-2">
      <Input
        type="search"
        placeholder="Search name, description, source..."
        value={props.query}
        onChange={(e: ChangeEvent<HTMLInputElement>) => props.setQuery(e.target.value)}
        className="min-w-[220px] flex-1"
      />
      <Select value={props.type} onValueChange={props.setType}>
        <SelectTrigger className="w-[130px]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {TYPES.map((t) => (
            <SelectItem key={t} value={t}>
              {typeLabel(t)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={props.platform} onValueChange={props.setPlatform}>
        <SelectTrigger className="w-[150px]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {PLATFORMS.map((p) => (
            <SelectItem key={p} value={p}>
              {p === "all" ? "All agents" : platformLabel(p)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={props.scope} onValueChange={props.setScope}>
        <SelectTrigger className="w-[130px]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {SCOPES.map((s) => (
            <SelectItem key={s} value={s}>
              {s === "all" ? "All scopes" : cap(s)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={props.sort} onValueChange={props.setSort}>
        <SelectTrigger className="w-[150px]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="name">Sort: Name</SelectItem>
          <SelectItem value="type">Sort: Type</SelectItem>
          <SelectItem value="platform">Sort: Agent</SelectItem>
          <SelectItem value="drift">Sort: Drift first</SelectItem>
        </SelectContent>
      </Select>
      <div className="flex gap-1">
        <Button
          type="button"
          variant={props.view === "cards" ? "secondary" : "outline"}
          size="icon"
          onClick={() => props.setView("cards")}
          aria-label="Card view"
          aria-pressed={props.view === "cards"}
        >
          <LayoutGrid className="h-4 w-4" />
        </Button>
        <Button
          type="button"
          variant={props.view === "table" ? "secondary" : "outline"}
          size="icon"
          onClick={() => props.setView("table")}
          aria-label="Table view"
          aria-pressed={props.view === "table"}
        >
          <List className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}
