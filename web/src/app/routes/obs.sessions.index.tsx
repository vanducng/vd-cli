import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";

import { TopBar } from "@/components/layout/top-bar";
import {
  agentFilterSchema,
  SessionsFilterBar,
  SessionsTable,
  SINCE_OPTIONS,
  sessionsQuery,
  type SessionsFilterValue,
} from "@/features/obs";

const PAGE_SIZE = 25;

const searchSchema = z.object({
  q: z.string().catch("").default(""),
  agent: agentFilterSchema.catch("all").default("all"),
  since: z.enum(SINCE_OPTIONS).catch("7d").default("7d"),
  project: z.string().catch("").default(""),
  offset: z.number().int().nonnegative().catch(0).default(0),
});

export const Route = createFileRoute("/obs/sessions/")({
  validateSearch: searchSchema,
  component: SessionsIndexPage,
});

function SessionsIndexPage() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();

  const filterValue: SessionsFilterValue = {
    q: search.q,
    agent: search.agent,
    since: search.since,
    project: search.project,
  };

  const { data, isLoading, isFetching, error } = useQuery(
    sessionsQuery({
      q: search.q || undefined,
      agent: search.agent === "all" ? undefined : search.agent,
      since: search.since,
      project: search.project || undefined,
      limit: PAGE_SIZE,
      offset: search.offset,
    }),
  );

  const isFiltered = Boolean(search.q || search.agent !== "all" || search.project);

  function patchFilter(patch: Partial<SessionsFilterValue>) {
    navigate({ search: (prev) => ({ ...prev, ...patch, offset: 0 }) });
  }

  function clearFilters() {
    navigate({ search: () => ({ q: "", agent: "all", since: search.since, project: "", offset: 0 }) });
  }

  function goToOffset(next: number) {
    navigate({ search: (prev) => ({ ...prev, offset: next }) });
  }

  const total = data?.total ?? 0;

  return (
    <div>
      <TopBar
        title="Sessions"
        subtitle={`${total} session${total === 1 ? "" : "s"}${isFetching ? " · syncing…" : ""} · est $ = API-equivalent, not a bill`}
      />
      <SessionsTable
        sessions={data?.sessions ?? []}
        isLoading={isLoading}
        error={error as Error | null}
        isFiltered={isFiltered}
        onClearFilters={clearFilters}
        pageSize={PAGE_SIZE}
        offset={search.offset}
        total={total}
        onPrev={() => goToOffset(Math.max(0, search.offset - PAGE_SIZE))}
        onNext={() => goToOffset(search.offset + PAGE_SIZE)}
        toolbar={<SessionsFilterBar value={filterValue} onChange={patchFilter} />}
      />
    </div>
  );
}
