import { queryOptions } from "@tanstack/react-query";

import { apiClient } from "@/lib/api-client";
import {
  sessionDetailSchema,
  sessionListSchema,
  skillReportSchema,
  usageReportSchema,
  type SessionFilter,
  type SkillFilter,
  type UsageFilter,
} from "@/features/obs/schemas";

export interface SessionDetailFilter {
  turns?: number;
  offset?: number;
}

export const obsKeys = {
  all: ["obs"] as const,
  sessions: (filter: SessionFilter) => [...obsKeys.all, "sessions", filter] as const,
  session: (id: string, filter: SessionDetailFilter = {}) => [...obsKeys.all, "session", id, filter] as const,
  usage: (filter: UsageFilter) => [...obsKeys.all, "usage", filter] as const,
  skills: (filter: SkillFilter) => [...obsKeys.all, "skills", filter] as const,
};

// The API's 404 body is {"error": "obs: session not found"} (obs.ErrNotFound).
// apiClient throws a plain Error with that message, so detect it by content
// rather than status code (apiClient does not expose one).
export function isSessionNotFound(error: unknown): boolean {
  return error instanceof Error && error.message.includes("not found");
}

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

// Server paginates turns via ?turns=&offset= (never loads a multi-MB session
// whole). Callers grow `turns` to page in more, see obs.sessions.$id.tsx's
// "load more".
export function sessionQuery(id: string, filter: SessionDetailFilter = {}) {
  return queryOptions({
    queryKey: obsKeys.session(id, filter),
    queryFn: async () => {
      const qs = buildQuery({ turns: filter.turns, offset: filter.offset });
      const raw = await apiClient.get<unknown>(`/api/obs/sessions/${encodeURIComponent(id)}${qs}`);
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

export function skillsQuery(filter: SkillFilter) {
  return queryOptions({
    queryKey: obsKeys.skills(filter),
    queryFn: async () => {
      const qs = buildQuery({ agent: filter.agent, project: filter.project, since: filter.since });
      const raw = await apiClient.get<unknown>(`/api/obs/skills${qs}`);
      return skillReportSchema.parse(raw);
    },
  });
}
