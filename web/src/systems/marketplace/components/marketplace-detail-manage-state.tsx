import { AlertCircle, SlidersHorizontal } from "lucide-react";

import { Button, Spinner } from "@compozy/ui";

import { MarketplaceDetailRailCard } from "./marketplace-detail-shell";

interface MarketplaceDetailManageFallbackBodyProps {
  error?: Error | null;
  errorMessage: string;
  isLoading?: boolean;
  loadingMessage: string;
  onRetry: () => void;
  testId: string;
}

/** Shared Manage loading/error body while the installed record is unavailable. */
function MarketplaceDetailManageFallbackBody({
  error,
  errorMessage,
  isLoading = false,
  loadingMessage,
  onRetry,
  testId,
}: MarketplaceDetailManageFallbackBodyProps) {
  return (
    <div
      className="flex flex-col gap-2 px-3.5 py-1.5"
      data-testid={testId}
      role={isLoading ? "status" : "alert"}
    >
      <div className="flex items-center gap-2 text-small-body font-medium text-fg">
        {isLoading ? (
          <Spinner aria-hidden="true" className="size-3.5 text-subtle" />
        ) : (
          <AlertCircle aria-hidden="true" className="size-3.5 text-danger" />
        )}
        {isLoading ? loadingMessage : errorMessage}
      </div>
      {!isLoading && error?.message ? (
        <p className="text-form-hint text-muted">{error.message}</p>
      ) : null}
      {!isLoading ? (
        <div className="pb-1">
          <Button onClick={onRetry} size="sm" type="button" variant="outline">
            Retry management
          </Button>
        </div>
      ) : null}
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
