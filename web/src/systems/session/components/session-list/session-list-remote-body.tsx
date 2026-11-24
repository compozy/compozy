import { Button } from "@compozy/ui";

import {
  mergeSessionPages,
  ScopedComputerError,
  ScopedComputerSkeleton,
  type MergedSessionRow,
  type ScopedComputerFailureItem,
} from "@/systems/computers";

import { buildSessionListThreads } from "../../lib/session-list-threads";
import type { SessionPayload } from "../../types";

/** Merges local thread sessions with remote fan-out rows ordered by `updated_at` desc. */
export function mergeLocalAndRemoteThreads(
  localThreads: ReturnType<typeof buildSessionListThreads>,
  remoteSessions: readonly MergedSessionRow<SessionPayload>[]
): ReturnType<typeof buildSessionListThreads> {
  if (remoteSessions.length === 0) return localThreads;
  const grouped = new Map<
    string,
    { computer: { id: string; name: string; status: string }; sessions: SessionPayload[] }
  >();
  for (const row of remoteSessions) {
    const entry = grouped.get(row.computerId) ?? {
      computer: {
        id: row.computerId,
        name: row.computerName,
        status: row.offline ? "offline" : "connected",
      },
      sessions: [],
    };
    entry.sessions.push(row.session);
    grouped.set(row.computerId, entry);
  }
  const merged = mergeSessionPages([
    {
      computer: { id: "local", name: "This computer", status: "connected" },
      sessions: localThreads.flatMap(thread => [thread.root, ...thread.children]),
    },
    ...grouped.values(),
  ]);
  return buildSessionListThreads(
    merged.map(row => row.session),
    ""
  );
}

/** Per-computer loading skeletons, inline retryable failures, and oldest-cursor Load more. */
export function SessionListRemoteFooter({
  loadingComputers,
  failedComputers,
  hasMore,
  loadingMore,
  onLoadMore,
  testIdPrefix,
}: {
  loadingComputers: readonly { id: string; name: string }[];
  failedComputers: readonly ScopedComputerFailureItem[];
  hasMore: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
  testIdPrefix: string;
}) {
  return (
    <>
      {loadingComputers.map(computer => (
        <ScopedComputerSkeleton
          computerId={computer.id}
          computerName={computer.name}
          key={computer.id}
        />
      ))}
      {failedComputers.map(computer => (
        <ScopedComputerError
          computerId={computer.id}
          computerName={computer.name}
          key={computer.id}
          onRetry={computer.retry}
        />
      ))}
      {hasMore ? (
        <div className="px-1 py-1.5">
          <Button
            className="w-full"
            data-testid={`${testIdPrefix}-remote-load-more`}
            disabled={loadingMore}
            onClick={onLoadMore}
            size="xs"
            type="button"
            variant="secondary"
          >
            {loadingMore ? "Loading…" : "Load more"}
          </Button>
        </div>
      ) : null}
    </>
  );
}
