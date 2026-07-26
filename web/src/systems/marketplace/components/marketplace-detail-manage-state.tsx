import { AlertCircle } from "lucide-react";

import { Button, Section, Spinner } from "@agh/ui";

interface MarketplaceDetailManageStateProps {
  error?: Error | null;
  isLoading?: boolean;
  label: string;
  onRetry: () => void;
  testId: string;
}

function MarketplaceDetailManageState({
  error,
  isLoading = false,
  label,
  onRetry,
  testId,
}: MarketplaceDetailManageStateProps) {
  return (
    <Section label="Manage">
      <div
        className="flex flex-col gap-2 rounded-lg border border-line bg-canvas-soft px-4 py-3"
        data-testid={testId}
        role={isLoading ? "status" : "alert"}
      >
        <div className="flex items-center gap-2 text-small-body font-medium text-fg">
          {isLoading ? (
            <Spinner aria-hidden="true" className="size-3.5 text-subtle" />
          ) : (
            <AlertCircle aria-hidden="true" className="size-3.5 text-danger" />
          )}
          {isLoading ? `Loading ${label} management…` : `${label} management could not be loaded.`}
        </div>
        {!isLoading && error?.message ? (
          <p className="text-caption text-muted">{error.message}</p>
        ) : null}
        {!isLoading ? (
          <div>
            <Button onClick={onRetry} size="sm" type="button" variant="outline">
              Retry management
            </Button>
          </div>
        ) : null}
      </div>
    </Section>
  );
}

export { MarketplaceDetailManageState };
export type { MarketplaceDetailManageStateProps };
