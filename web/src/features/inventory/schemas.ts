import { z } from "zod";

// Mirrors internal/inventory/types.go AssetSummary. `omitempty` Go tags → .optional()
// (key absent when zero-valued); Platform/Enabled have no `omitempty` → always present.
export const assetTypeSchema = z.enum(["skill", "agent", "command", "hook", "rule"]);

export const assetSummarySchema = z.object({
  type: assetTypeSchema,
  name: z.string(),
  description: z.string(),
  source: z.string().optional(),
  mode: z.string().optional(),
  sha: z.string().optional(),
  drift: z.string().optional(),
  enabled: z.boolean(),
  platform: z.string(),
});

export const inventorySchema = z.object({
  managed: z.array(assetSummarySchema),
  discovered: z.array(assetSummarySchema),
});

// SkillDetail embeds AssetSummary (Go struct embedding flattens JSON fields).
export const skillDetailSchema = assetSummarySchema.extend({
  frontmatter: z.record(z.unknown()).optional(),
  body: z.string(),
  path: z.string(),
});

export type AssetType = z.infer<typeof assetTypeSchema>;
export type AssetSummary = z.infer<typeof assetSummarySchema>;
export type Inventory = z.infer<typeof inventorySchema>;
export type SkillDetail = z.infer<typeof skillDetailSchema>;
