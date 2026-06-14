import { platformLabel } from "./labels";

export function PlatformBadge({ platform }: { platform: string }) {
  if (!platform) return <span className="badge scope-managed">repo</span>;
  return <span className={`badge plat-${platform}`}>{platformLabel(platform)}</span>;
}
