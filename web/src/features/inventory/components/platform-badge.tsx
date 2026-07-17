import { Badge } from "@/components/ui/badge";
import { platformLabel } from "./labels";

export function PlatformBadge({ platform }: { platform: string }) {
  if (!platform) return <Badge variant="amber">repo</Badge>;
  if (platform === "claude_code") return <Badge variant="claude">{platformLabel(platform)}</Badge>;
  if (platform === "codex") return <Badge variant="codex">{platformLabel(platform)}</Badge>;
  // No design-contract token for cursor yet; keep the pre-migration accent color.
  return <Badge className="border-[#9b8cff]/45 text-[#9b8cff]">{platformLabel(platform)}</Badge>;
}
