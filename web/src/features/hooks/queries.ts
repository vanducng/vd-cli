import { useQuery } from "@tanstack/react-query";

import { apiClient } from "@/lib/api-client";
import { hooksResponseSchema } from "./schemas";

export const hooksKeys = {
  all: ["hooks"] as const,
  list: () => [...hooksKeys.all, "list"] as const,
};

export function useHooks() {
  return useQuery({
    queryKey: hooksKeys.list(),
    queryFn: async () => hooksResponseSchema.parse(await apiClient.get<unknown>("/api/hooks")).hooks,
  });
}
