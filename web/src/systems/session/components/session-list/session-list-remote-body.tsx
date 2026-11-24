import { Button } from "@compozy/ui";

import {
  ScopedComputerError,
  ScopedComputerSkeleton,
  type ScopedComputerFailureItem,
} from "@/systems/computers";

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
