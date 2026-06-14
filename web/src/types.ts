export type AssetType = "skill" | "agent" | "command" | "hook" | "rule";

export interface AssetSummary {
  type: AssetType;
  name: string;
  description: string;
  source?: string;
  mode?: string;
  sha?: string;
  drift?: string;
  enabled: boolean;
  platform: string;
}

export interface Inventory {
  managed: AssetSummary[];
  discovered: AssetSummary[];
}

export interface SkillDetail extends AssetSummary {
  frontmatter?: Record<string, unknown>;
  body: string;
  path: string;
}

export interface HookAsset {
  type: string;
  name: string;
  description: string;
  enabled: boolean;
  path: string;
  frontmatter?: Record<string, unknown>;
}

export interface DoctorEntry {
  skill: string;
  status: string;
  detail: string;
}

export interface DoctorReport {
  entries: DoctorEntry[];
}
