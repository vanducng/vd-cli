import { z } from "zod";

// Mirrors internal/inventory/types.go DoctorEntry/DoctorReport. No omitempty tags →
// all fields always present (Status is a DriftStatus.String(), never omitted).
export const doctorEntrySchema = z.object({
  skill: z.string(),
  status: z.string(),
  detail: z.string(),
});

export const doctorReportSchema = z.object({
  entries: z.array(doctorEntrySchema),
});

export type DoctorEntry = z.infer<typeof doctorEntrySchema>;
