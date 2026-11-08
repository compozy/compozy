import { AlertCircle } from "@compozy/ui/icons";

import { Button, Empty } from "@compozy/ui";

interface MarketplaceCatalogFailureProps {
  error: Error;
  onRetry: () => void;
  testId: string;
  retryTestId?: string;
  /** The daemon answered and reported its sources unreachable, rather than the read failing. */
  unreachable: boolean;
}

/**
 * A catalog read with nothing to show. Only the daemon's own source report means the marketplace
 * is unreachable; a failed request (a 5xx, a bad response, no daemon) is a load failure, so the
 * title never blames sources that may be fine.
 */
function MarketplaceCatalogFailure({
  error,
  onRetry,
  retryTestId,
  testId,
  unreachable,
}: MarketplaceCatalogFailureProps) {
  return (
    <Empty
      action={
        <Button
          data-testid={retryTestId}
          onClick={onRetry}
          size="sm"
          type="button"
          variant="secondary"
        >
          Retry
        </Button>
      }
      cause={error.message}
      data-testid={testId}
      description={
        unreachable
          ? "No sources responded. Retry, or come back in a moment."
          : "Something went wrong while loading the catalog. Retry, or come back in a moment."
      }
      framed
      icon={AlertCircle}
      title={unreachable ? "The marketplace is unreachable" : "Couldn't load the marketplace"}
      titleAs="h2"
    />
  );
}

export { MarketplaceCatalogFailure };
