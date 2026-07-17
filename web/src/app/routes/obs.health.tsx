import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";

import { TopBar } from "@/components/layout/top-bar";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { HealthClustersTable, ToolErrorBars, healthQuery, type Agent } from "@/features/obs";
import { formatCount, formatPct } from "@/features/obs/lib/format";
import { KpiStrip, type Kpi } from "@/features/shared/components/kpi-strip";

const searchSchema = z.object({
  agent: z.enum(["all", "claude-code", "codex"]).catch("all").default("all"),
  // Restrict to the dropdown options so a hand-edited ?since= can't reach the API.
  since: z.enum(["7d", "30d", "90d", "0d"]).catch("7d").default("7d"),
  project: z.string().catch("").default(""),
});

export const Route = createFileRoute("/obs/health")({
  validateSearch: searchSchema,
  component: HealthPage,
});

function HealthPage() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();

  const { data, isLoading, error } = useQuery(
    healthQuery({
      agent: search.agent === "all" ? undefined : (search.agent as Agent),
      since: search.since,
      project: search.project || undefined,
    }),
  );
  const queryError = error as Error | null;

  const topTool = data?.bytool[0];
  const kpis: Kpi[] = [
    {
      label: "Total errors",
      value: data ? formatCount(data.totalerrors) : "?",
      sublabel:
        data == null
          ? undefined
          : data.delta == null
            ? "low sample"
            : `${data.delta >= 0 ? "+" : ""}${formatCount(data.delta)} vs prior ${search.since}`,
    },
    {
      label: "Error rate",
      value: data ? formatPct(data.errorrate) : "?",
      sublabel: "of all tool calls",
    },
    {
      label: "Errored sessions",
      value: data ? formatCount(data.erroredsessions) : "?",
    },
    {
      label: "Top offender",
      value: topTool ? topTool.tool : "—",
      sublabel: topTool ? `${formatCount(topTool.count)} errors` : undefined,
    },
  ];

  return (
    <div>
      <TopBar
        title="Health"
        subtitle="Investigate signals, not verdicts — agents fail-probe routinely; a count says look here, never this is broken."
      />

      <div className="sticky top-14 z-20 -mx-1 mb-4 flex flex-wrap items-center gap-2 bg-background/90 px-1 py-2 backdrop-blur">
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
        <Select
          value={search.since}
          onValueChange={(v) => navigate({ search: (prev) => ({ ...prev, since: v as typeof search.since }) })}
        >
          <SelectTrigger className="w-[140px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="7d">last 7d</SelectItem>
            <SelectItem value="30d">last 30d</SelectItem>
            <SelectItem value="90d">last 90d</SelectItem>
            <SelectItem value="0d">all time</SelectItem>
          </SelectContent>
        </Select>
        <Input
          placeholder="Project: all"
          value={search.project}
          onChange={(e) => navigate({ search: (prev) => ({ ...prev, project: e.target.value }) })}
          className="w-[160px]"
        />
      </div>

      <KpiStrip items={kpis} />

      <div className="mb-4">
        <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-faint">Errors by tool</div>
        <ToolErrorBars items={data?.bytool ?? []} isLoading={isLoading} error={queryError} />
      </div>

      <HealthClustersTable clusters={data?.clusters ?? []} isLoading={isLoading} error={queryError} />
    </div>
  );
}
