import { useState } from "react";
import { Link } from "@tanstack/react-router";

import { Button, SkeletonRegion, SkeletonText } from "@compozy/ui";

import { useGoalVerificationSessions } from "../../hooks/use-goal-verification-sessions";

function VerificationSessionList({ sessionId }: { sessionId: string }) {
  const query = useGoalVerificationSessions(sessionId);
  return (
    <span className="flex flex-col gap-1">
      {query.isPending ? (
        <SkeletonRegion label="Loading verification sessions" className="flex flex-col gap-1">
          <SkeletonText className="w-44" />
          <SkeletonText className="w-36" />
        </SkeletonRegion>
      ) : null}
      {query.isError ? (
        <Button variant="ghost" size="xs" onClick={() => void query.refetch()}>
          Retry verification sessions
        </Button>
      ) : null}
      {query.isSuccess && query.sessions.length === 0 ? (
        <span>No verification sessions recorded.</span>
      ) : null}
      {query.sessions.map(session => (
        <Link
          key={session.id}
          to="/session/$id"
          params={{ id: session.id }}
          className="text-info hover:underline hover:underline-offset-2"
        >
          {session.name || session.id} · {session.state}
        </Link>
      ))}
      {query.hasNextPage ? (
        <Button
          variant="ghost"
          size="xs"
          disabled={query.isFetchingNextPage}
          onClick={() => void query.fetchNextPage()}
        >
          Load more verification sessions
        </Button>
      ) : null}
    </span>
  );
}

export function GoalVerificationSessions({ sessionId }: { sessionId: string }) {
  const [open, setOpen] = useState(false);
  return (
    <span className="flex flex-col gap-1">
      <Button
        variant="ghost"
        size="xs"
        aria-expanded={open}
        onClick={() => setOpen(value => !value)}
      >
        Verification sessions
      </Button>
      {open ? <VerificationSessionList sessionId={sessionId} /> : null}
    </span>
  );
}
