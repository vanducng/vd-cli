import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";

import { TopBar } from "@/components/layout/top-bar";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { UsageChart, UsageTable, usageQuery, type Agent } from "@/features/obs";

const GROUP_OPTIONS = ["daily", "monthly"] as const;

const searchSchema = z.object({
  group: z.enum(GROUP_OPTIONS).catch("daily").default("daily"),
  agent: z.enum(["all", "claude-code", "codex"]).catch("all").default("all"),
  since: z.string().catch("7d").default("7d"),
});

export const Route = createFileRoute("/obs/usage")({
  validateSearch: searchSchema,
  component: UsagePage,
});

function UsagePage() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();

  const { data, isLoading, error } = useQuery(
    usageQuery({
      group: search.group,
      agent: search.agent === "all" ? undefined : (search.agent as Agent),
      since: search.since,
    }),
  );

  const unpriced = data?.unpricedmodels ?? [];

  return (
    <div>
      <TopBar
        title="Usage"
        subtitle={`last ${search.since} · grouped ${search.group} · est $ = API-equivalent from token counts, not a subscription bill`}
      />

      <div className="mb-4 flex gap-2">
        <Select
          value={search.group}
          onValueChange={(v) => navigate({ search: (prev) => ({ ...prev, group: v as (typeof GROUP_OPTIONS)[number] }) })}
        >
          <SelectTrigger className="w-[140px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {GROUP_OPTIONS.map((g) => (
              <SelectItem key={g} value={g}>
                {g === "daily" ? "Daily" : "Monthly"}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select
          value={search.agent}
          onValueChange={(v) => navigate({ search: (prev) => ({ ...prev, agent: v as typeof search.agent }) })}
        >
          <SelectTrigger className="w-[160px]">
            <SelectValue placeholder="Agent: all" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Agent: all</SelectItem>
            <SelectItem value="claude-code">claude-code</SelectItem>
            <SelectItem value="codex">codex</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {unpriced.length > 0 && (
        <div className="mb-3 rounded-sm border border-primary/45 bg-primary/[0.08] px-3 py-2 text-sm text-primary">
          ! {unpriced.length} unpriced model{unpriced.length === 1 ? "" : "s"}: {unpriced.join(", ")}, add to
          ~/.vd/obs/prices.json
        </div>
      )}

      <UsageChart rows={data?.rows ?? []} isLoading={isLoading} error={error as Error | null} />

      <div className="h-4" />

      <UsageTable
        rows={data?.rows ?? []}
        totals={
          data?.totals ?? { input: 0, output: 0, cacheread: 0, cachewrite: 0, reasoningoutput: 0 }
        }
        totalCostUsd={data?.totalcostusd ?? null}
        isLoading={isLoading}
        error={error as Error | null}
      />

      <p className="mt-2 text-xs text-faint">Unpriced models render "?", never $0.00; a fake zero reads as free.</p>
    </div>
  );
}
