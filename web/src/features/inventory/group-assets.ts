import type { AssetType } from "./schemas";
import type { Row } from "./components/labels";

export interface GroupedAsset {
  type: AssetType;
  name: string;
  description: string;
  enabled: boolean;
  platforms: string[];
  managed: boolean;
  source?: string;
  sha?: string;
  drift?: string;
}

// One skill/agent/etc. can appear as separate rows per platform (claude_code, codex)
// and/or once more under "managed"; collapse those into a single card-worthy entry.
export function groupAssets(rows: Row[]): GroupedAsset[] {
  const map = new Map<string, GroupedAsset>();

  for (const r of rows) {
    const key = `${r.type}/${r.name}`;
    const g = map.get(key) ?? {
      type: r.type,
      name: r.name,
      description: r.description,
      enabled: r.enabled,
      platforms: [],
      managed: false,
    };
    if (r.platform && !g.platforms.includes(r.platform)) g.platforms.push(r.platform);
    if (r.scope === "managed") g.managed = true;
    if (!g.description && r.description) g.description = r.description;
    if (!g.source && r.source) g.source = r.source;
    if (!g.sha && r.sha) g.sha = r.sha;
    if (!g.drift && r.drift) g.drift = r.drift;
    if (r.enabled) g.enabled = true;
    map.set(key, g);
  }

  return [...map.values()];
}
