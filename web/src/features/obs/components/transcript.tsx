import { Button } from "@/components/ui/button";
import { SessionHeader } from "@/features/obs/components/session-header";
import { TurnCard } from "@/features/obs/components/turn-card";
import type { SessionDetail } from "@/features/obs/schemas";

interface TranscriptProps {
  session: SessionDetail;
  isFetchingMore?: boolean;
  onLoadMore: () => void;
}

/** The session transcript: header stats, then every loaded turn. The payoff
 * view of the whole portal, since a terminal can't scroll a 40-turn session
 * with tool blocks and rollups; this can. */
export function Transcript({ session, isFetchingMore, onLoadMore }: TranscriptProps) {
  const hasMore = session.turns.length < session.turncount;

  return (
    <div>
      <SessionHeader session={session} />

      {session.turns.length === 0 ? (
        <p className="py-8 text-center text-sm text-muted-foreground">This session has no turns yet.</p>
      ) : (
        <ol className="grid gap-5" aria-label="Turn timeline">
          {session.turns.map((turn, i) => (
            <TurnCard
              key={turn.id}
              turn={turn}
              agent={session.agent}
              sessionWorkflowId={session.workflowid}
              isLast={i === session.turns.length - 1}
            />
          ))}
        </ol>
      )}

      {hasMore && (
        <div className="flex items-center justify-between pt-2 text-sm text-muted-foreground">
          <span>
            Showing {session.turns.length} of {session.turncount} turns
          </span>
          <Button variant="outline" size="sm" onClick={onLoadMore} disabled={isFetchingMore}>
            {isFetchingMore ? "Loading…" : "Load more"}
          </Button>
        </div>
      )}
    </div>
  );
}
