import { useEffect, useState } from "react";

import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { agentSchema, type Agent } from "@/features/obs/schemas";

export const SINCE_OPTIONS = ["24h", "7d", "30d"] as const;
export type SinceOption = (typeof SINCE_OPTIONS)[number];

export interface SessionsFilterValue {
  q: string;
  agent: Agent | "all";
  since: SinceOption;
  project: string;
}

interface SessionsFilterBarProps {
  value: SessionsFilterValue;
  onChange: (patch: Partial<SessionsFilterValue>) => void;
}

/** Compact toolbar: search + agent + since + project. Every control maps 1:1 to
 * an API query param; filtering happens server-side, the session set is large
 * (fastreact rule 7/8: no wrapping pill rows). */
export function SessionsFilterBar({ value, onChange }: SessionsFilterBarProps) {
  const [search, setSearch] = useState(value.q);

  useEffect(() => setSearch(value.q), [value.q]);

  useEffect(() => {
    const t = setTimeout(() => {
      if (search !== value.q) onChange({ q: search });
    }, 300);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search]);

  return (
    <div className="flex flex-1 flex-wrap gap-2">
      <Input
        type="search"
        placeholder="Search title or cwd…"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        className="min-w-[220px] flex-1"
      />
      <Select value={value.agent} onValueChange={(v) => onChange({ agent: v as SessionsFilterValue["agent"] })}>
        <SelectTrigger className="w-[160px]">
          <SelectValue placeholder="Agent: all" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">Agent: all</SelectItem>
          {agentSchema.options.map((a) => (
            <SelectItem key={a} value={a}>
              {a}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={value.since} onValueChange={(v) => onChange({ since: v as SinceOption })}>
        <SelectTrigger className="w-[130px]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {SINCE_OPTIONS.map((s) => (
            <SelectItem key={s} value={s}>
              Since: {s}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Input
        placeholder="Project: all"
        value={value.project}
        onChange={(e) => onChange({ project: e.target.value })}
        className="w-[160px]"
      />
    </div>
  );
}
