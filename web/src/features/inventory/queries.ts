import { useQuery } from "@tanstack/react-query";

import { apiClient } from "@/lib/api-client";
import { inventorySchema, skillDetailSchema } from "./schemas";

export const inventoryKeys = {
  all: ["inventory"] as const,
  list: () => [...inventoryKeys.all, "list"] as const,
  detail: (name: string) => [...inventoryKeys.all, "detail", name] as const,
};

export function useInventory() {
  return useQuery({
    queryKey: inventoryKeys.list(),
    queryFn: async () => inventorySchema.parse(await apiClient.get<unknown>("/api/inventory")),
  });
}

export function useSkillDetail(name: string | null) {
  return useQuery({
    queryKey: inventoryKeys.detail(name ?? ""),
    queryFn: async () =>
      skillDetailSchema.parse(await apiClient.get<unknown>(`/api/skills/${encodeURIComponent(name as string)}`)),
    enabled: name != null,
  });
}
