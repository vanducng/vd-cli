import { z } from "zod";

// Hooks() returns []inventory.Asset (internal/inventory/asset.go), NOT AssetSummary:
// it carries Path/Frontmatter instead of Source/Mode/SHA/Drift.
export const hookAssetSchema = z.object({
  type: z.string(),
  name: z.string(),
  description: z.string(),
  enabled: z.boolean(),
  path: z.string(),
  frontmatter: z.record(z.unknown()).optional(),
  platform: z.string(),
});

export const hooksResponseSchema = z.object({
  hooks: z.array(hookAssetSchema),
});

export type HookAsset = z.infer<typeof hookAssetSchema>;
