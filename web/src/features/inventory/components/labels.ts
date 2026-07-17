import type { AssetSummary, AssetType } from "../schemas";

export type Scope = "managed" | "discovered";

export interface Row extends AssetSummary {
  scope: Scope;
}

export const TYPES: (AssetType | "all")[] = ["all", "skill", "agent", "command", "rule"];
export const PLATFORMS = ["all", "claude_code", "codex", "cursor"] as const;
export const SCOPES = ["all", "managed", "discovered"] as const;

export function platformLabel(p: string): string {
  switch (p) {
    case "claude_code":
      return "Claude Code";
    case "codex":
      return "Codex";
    case "cursor":
      return "Cursor";
    case "":
      return "repo";
    default:
      return p;
  }
}

export function typeLabel(t: string): string {
  if (t === "all") return "All types";
  return t.charAt(0).toUpperCase() + t.slice(1) + "s";
}

export function cap(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

export function hasDrift(d?: string): boolean {
  return !!d && d !== "none";
}
