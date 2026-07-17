import { useQuery } from "@tanstack/react-query";

import { apiClient } from "@/lib/api-client";
import { doctorReportSchema } from "./schemas";

export const doctorKeys = {
  all: ["doctor"] as const,
  report: () => [...doctorKeys.all, "report"] as const,
};

export function useDoctorReport() {
  return useQuery({
    queryKey: doctorKeys.report(),
    queryFn: async () => doctorReportSchema.parse(await apiClient.get<unknown>("/api/doctor")).entries,
  });
}
