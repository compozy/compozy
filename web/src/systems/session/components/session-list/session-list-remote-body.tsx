import { Button } from "@compozy/ui";

import {
  ScopedComputerError,
  ScopedComputerSkeleton,
  type ScopedComputerFailureItem,
} from "@/systems/computers";

/** Per-computer loading skeletons, inline retryable failures, and Load more naming the next computer (`F6`, `S10`). */
export function SessionListRemoteFooter({
  loadingComputers,
  failedComputers,
  hasMore,
  loadingMore,
  loadMoreLabel,
  onLoadMore,
  testIdPrefix,
}: {
  loadingComputers: readonly { id: string; name: string }[];
  failedComputers: readonly ScopedComputerFailureItem[];
  hasMore: boolean;
  loadingMore: boolean;
  loadMoreLabel?: string;
  onLoadMore: () => void;
  testIdPrefix: string;
}) {
  return (
    <>
      {loadingComputers.map(computer => (
        <ScopedComputerSkeleton
          className="mx-1"
          computerId={computer.id}
          computerName={computer.name}
          key={computer.id}
        />
      ))}
      {failedComputers.map(computer => (
        <ScopedComputerError
          className="mx-1 mt-1"
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
            {loadingMore ? "Loading…" : (loadMoreLabel ?? "Load more")}
          </Button>
        </div>
      ) : null}
    </>
  );
}
