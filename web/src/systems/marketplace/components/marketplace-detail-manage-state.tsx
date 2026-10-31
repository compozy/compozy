import { AlertCircle, SlidersHorizontal } from "@compozy/ui/icons";

import { Button, PropertyRowSkeleton, SkeletonRegion } from "@compozy/ui";

import { MarketplaceDetailRailCard } from "./marketplace-detail-shell";

interface MarketplaceDetailManageFallbackBodyProps {
  error?: Error | null;
  errorMessage: string;
  isLoading?: boolean;
  loadingMessage: string;
  onRetry: () => void;
  testId: string;
}

/**
 * Shared Manage loading/error body while the installed record is unavailable. Loading holds the
 * card's three property rows (Enabled, trust, placement); a failure names itself and offers retry.
 */
function MarketplaceDetailManageFallbackBody({
  error,
  errorMessage,
  isLoading = false,
  loadingMessage,
  onRetry,
  testId,
}: MarketplaceDetailManageFallbackBodyProps) {
  if (isLoading) {
    return (
      <SkeletonRegion className="px-3.5" data-testid={testId} label={loadingMessage}>
        <PropertyRowSkeleton count={3} />
      </SkeletonRegion>
    );
  }
  return (
    <div className="flex flex-col gap-2 px-3.5 py-1.5" data-testid={testId} role="alert">
      <div className="flex items-center gap-2 text-small-body font-medium text-fg">
        <AlertCircle aria-hidden="true" className="size-3.5 text-danger" />
        {errorMessage}
      </div>
      {error?.message ? <p className="text-form-hint text-muted">{error.message}</p> : null}
      <div className="pb-1">
        <Button onClick={onRetry} size="sm" type="button" variant="secondary">
          Retry management
        </Button>
      </div>
    </div>
  );
}

interface MarketplaceDetailManageFallbackCardProps {
  error?: Error | null;
  isLoading?: boolean;
  label: string;
  onRetry: () => void;
  testId: string;
}

/** Rail Manage card body while the installed record is loading or failed to load. */
function MarketplaceDetailManageFallbackCard({
  error,
  isLoading = false,
  label,
  onRetry,
  testId,
}: MarketplaceDetailManageFallbackCardProps) {
  return (
    <MarketplaceDetailRailCard icon={SlidersHorizontal} title="Manage">
      <MarketplaceDetailManageFallbackBody
        error={error}
        errorMessage={`${label} management could not be loaded.`}
        isLoading={isLoading}
        loadingMessage={`Loading ${label} management…`}
        onRetry={onRetry}
        testId={testId}
      />
    </MarketplaceDetailRailCard>
  );
}

export { MarketplaceDetailManageFallbackBody, MarketplaceDetailManageFallbackCard };
export type { MarketplaceDetailManageFallbackBodyProps, MarketplaceDetailManageFallbackCardProps };
