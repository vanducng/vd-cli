import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";

import { TopBar } from "@/components/layout/top-bar";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { SkillsTable, skillsQuery, type Agent } from "@/features/obs";

const searchSchema = z.object({
  agent: z.enum(["all", "claude-code", "codex"]).catch("all").default("all"),
  since: z.string().catch("30d").default("30d"),
});

export const Route = createFileRoute("/obs/skills")({
  validateSearch: searchSchema,
  component: SkillsPage,
});

function SkillsPage() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();

  const { data, isLoading, error } = useQuery(
    skillsQuery({
      agent: search.agent === "all" ? undefined : (search.agent as Agent),
      since: search.since,
    }),
  );

  return (
    <div>
      <TopBar
        title="Skills"
        subtitle="per-invocation attribution: a skill owns the turns from its invocation to the next · (none) = pre-invocation or no-skill activity"
      />

      <div className="mb-4 flex gap-2">
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
          onValueChange={(v) => navigate({ search: (prev) => ({ ...prev, since: v }) })}
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
      </div>

      <SkillsTable skills={data?.skills ?? []} isLoading={isLoading} error={error as Error | null} />

      <p className="mt-2 text-xs text-faint">
        CORR = user push-backs · ABRT = interrupt marker · counters flag candidates, not proven fault.
      </p>
    </div>
  );
}
