import { Badge } from "@/components/ui/badge";
import { platformLabel } from "./labels";

export function PlatformBadge({ platform }: { platform: string }) {
  if (!platform) return <Badge variant="amber">repo</Badge>;
  if (platform === "claude_code") return <Badge variant="claude">{platformLabel(platform)}</Badge>;
  if (platform === "codex") return <Badge variant="codex">{platformLabel(platform)}</Badge>;
  // No design-contract token for cursor yet; keep the pre-migration accent color.
  return <Badge className="border-[#9b8cff]/45 text-[#9b8cff]">{platformLabel(platform)}</Badge>;
}

function GhostBadge({ platform }: { platform: string }) {
  return (
    <Badge className="border-dashed border-border bg-transparent text-faint">{platformLabel(platform)}</Badge>
  );
}

/** Dual claude/codex badges for one grouped asset: solid when the platform is present,
 * dashed ghost when it's not — per the winning mock's dedup card treatment. */
export function PlatformBadgeRow({ platforms }: { platforms: string[] }) {
  const extra = platforms.filter((p) => p !== "claude_code" && p !== "codex");
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {platforms.includes("claude_code") ? (
        <PlatformBadge platform="claude_code" />
      ) : (
        <GhostBadge platform="claude_code" />
      )}
      {platforms.includes("codex") ? <PlatformBadge platform="codex" /> : <GhostBadge platform="codex" />}
      {extra.map((p) => (
        <PlatformBadge key={p} platform={p} />
      ))}
    </div>
  );
}
