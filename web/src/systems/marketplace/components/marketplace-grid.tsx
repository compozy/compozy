import { Skeleton } from "@agh/ui";

import type { MarketplaceListing } from "../types";
import { MarketplaceCard } from "./marketplace-card";

interface MarketplaceGridProps {
  entries: readonly MarketplaceListing[];
  isEntryPending?: (entry: MarketplaceListing) => boolean;
  onAction: (entry: MarketplaceListing) => void;
}

function MarketplaceGrid({ entries, isEntryPending, onAction }: MarketplaceGridProps) {
  return (
    <div
      className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3"
      data-testid="marketplace-grid"
      data-view="cards"
    >
      {entries.map(entry => (
        <MarketplaceCard
          entry={entry}
          key={`${entry.kind}:${entry.entry_id}`}
          onAction={onAction}
          pending={isEntryPending?.(entry) ?? false}
        />
      ))}
    </div>
  );
}

function MarketplaceGridSkeleton({ count = 6 }: { count?: number }) {
  return (
    <div
      aria-label="Loading marketplace entries"
      className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3"
      role="status"
    >
      {Array.from({ length: count }, (_, index) => (
        <div className="flex min-h-38 flex-col rounded-lg bg-canvas-soft p-4" key={index}>
          <Skeleton className="h-2.5 w-3/5" />
          <Skeleton className="mt-1.5 h-2.5 w-5/6" />
          <Skeleton className="mt-auto h-2.5 w-2/5" />
        </div>
      ))}
    </div>
  );
}

export { MarketplaceGrid, MarketplaceGridSkeleton };
export type { MarketplaceGridProps };
