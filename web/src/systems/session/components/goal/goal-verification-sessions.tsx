import { useState } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import { Button } from "@compozy/ui";

import { sessionsListOptions } from "../../lib/query-options";
import { flattenSessionPages } from "../../lib/session-list-query";

function VerificationSessionList({ sessionId }: { sessionId: string }) {
  const query = useInfiniteQuery({
    ...sessionsListOptions({ parent: sessionId, type: "system", all_profiles: true }),
    refetchInterval: 5_000,
  });
  const sessions = flattenSessionPages(query.data) ?? [];
  return (
    <div className="flex flex-col gap-1">
      {query.isPending ? <span>Loading verification sessions…</span> : null}
      {query.isError ? (
        <Button variant="ghost" size="xs" onClick={() => void query.refetch()}>
          Retry verification sessions
        </Button>
      ) : null}
      {query.isSuccess && sessions.length === 0 ? (
        <span>No verification sessions recorded.</span>
      ) : null}
      {sessions.map(session => (
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
    </div>
  );
}

export function GoalVerificationSessions({ sessionId }: { sessionId: string }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="flex flex-col gap-1">
      <Button
        variant="ghost"
        size="xs"
        aria-expanded={open}
        onClick={() => setOpen(value => !value)}
      >
        Verification sessions
      </Button>
      {open ? <VerificationSessionList sessionId={sessionId} /> : null}
    </div>
  );
}
