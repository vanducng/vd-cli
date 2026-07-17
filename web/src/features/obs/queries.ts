import { queryOptions } from "@tanstack/react-query";

import { apiClient } from "@/lib/api-client";
import {
  sessionDetailSchema,
  sessionListSchema,
  usageReportSchema,
  type SessionFilter,
  type UsageFilter,
} from "@/features/obs/schemas";

export const obsKeys = {
  all: ["obs"] as const,
  sessions: (filter: SessionFilter) => [...obsKeys.all, "sessions", filter] as const,
  session: (id: string) => [...obsKeys.all, "session", id] as const,
  usage: (filter: UsageFilter) => [...obsKeys.all, "usage", filter] as const,
};

function buildQuery(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === "") continue;
    search.set(key, String(value));
  }
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

export function sessionsQuery(filter: SessionFilter) {
  return queryOptions({
    queryKey: obsKeys.sessions(filter),
    queryFn: async () => {
      const qs = buildQuery({
        agent: filter.agent,
        project: filter.project,
        q: filter.q,
        since: filter.since,
        limit: filter.limit,
        offset: filter.offset,
        sort: filter.sort,
      });
      const raw = await apiClient.get<unknown>(`/api/obs/sessions${qs}`);
      return sessionListSchema.parse(raw);
    },
  });
}

export function sessionQuery(id: string) {
  return queryOptions({
    queryKey: obsKeys.session(id),
    queryFn: async () => {
      const raw = await apiClient.get<unknown>(`/api/obs/sessions/${encodeURIComponent(id)}`);
      return sessionDetailSchema.parse(raw);
    },
    enabled: Boolean(id),
  });
}

export function usageQuery(filter: UsageFilter) {
  return queryOptions({
    queryKey: obsKeys.usage(filter),
    queryFn: async () => {
      const qs = buildQuery({ group: filter.group, agent: filter.agent, since: filter.since });
      const raw = await apiClient.get<unknown>(`/api/obs/usage${qs}`);
      return usageReportSchema.parse(raw);
    },
  });
}
