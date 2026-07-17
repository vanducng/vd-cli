import { Badge } from "@/components/ui/badge";

const LABELS: Record<string, string> = {
  "claude-code": "claude",
  codex: "codex",
};

const VARIANTS: Record<string, "claude" | "codex" | "default"> = {
  "claude-code": "claude",
  codex: "codex",
};

/** Renders the agent pill used across sessions + usage tables. Falls back to the
 * raw value for an agent value neither surface has seen yet. */
export function AgentBadge({ agent }: { agent: string }) {
  return <Badge variant={VARIANTS[agent] ?? "default"}>{LABELS[agent] ?? agent}</Badge>;
}
