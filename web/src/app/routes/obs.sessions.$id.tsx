import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";

import { Skeleton } from "@/components/ui/skeleton";
import { Transcript } from "@/features/obs/components/transcript";
import { isSessionNotFound, sessionQuery } from "@/features/obs";

const PAGE_SIZE = 30;

export const Route = createFileRoute("/obs/sessions/$id")({
  component: SessionDetailPage,
});

function SessionDetailPage() {
  const { id } = Route.useParams();
  const [visibleTurns, setVisibleTurns] = useState(PAGE_SIZE);

  const { data, isLoading, isFetching, error } = useQuery(sessionQuery(id, { turns: visibleTurns }));

  if (error) {
    if (isSessionNotFound(error)) {
      return (
        <div className="py-16 text-center">
          <p className="text-lg font-semibold">Session not found</p>
          <p className="mt-1 text-sm text-muted-foreground">No session matches &ldquo;{id}&rdquo;.</p>
          <Link to="/obs/sessions" className="mt-4 inline-block text-sm text-info hover:underline">
            ‹ back to sessions
          </Link>
        </div>
      );
    }
    return (
      <div className="py-16 text-center">
        <p className="text-sm text-err">{(error as Error).message}</p>
      </div>
    );
  }

  if (isLoading || !data) {
    return (
      <div className="grid gap-3">
        <Skeleton className="h-28 w-full" />
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-32 w-full" />
      </div>
    );
  }

  return (
    <Transcript
      session={data}
      isFetchingMore={isFetching}
      onLoadMore={() => setVisibleTurns((n) => n + PAGE_SIZE)}
    />
  );
}
