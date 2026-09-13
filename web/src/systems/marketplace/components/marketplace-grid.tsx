import type { ReactNode } from "react";

import { cn, Skeleton } from "@compozy/ui";

interface MarketplaceGridProps {
  children: ReactNode;
  className?: string;
  "data-testid"?: string;
}

/** Row-card grid: two columns from 960px of window width (container query), one below. */
function MarketplaceGrid({
  children,
  className,
  "data-testid": testId = "marketplace-grid",
}: MarketplaceGridProps) {
  return (
    <div
      className={cn("grid grid-cols-1 gap-2 @min-[960px]:grid-cols-2", className)}
      data-testid={testId}
      data-view="cards"
    >
      {children}
    </div>
  );
}

const SKELETON_WIDTHS = [
  [38, 78],
  [52, 64],
  [30, 84],
  [44, 70],
  [36, 76],
  [48, 62],
] as const;

/** Skeleton rows in the real grid: static blocks, no shimmer — the row grammar is the signal. */
function MarketplaceGridSkeleton({ count = 6 }: { count?: number }) {
  return (
    <MarketplaceGrid data-testid="marketplace-grid-skeleton">
      {Array.from({ length: count }, (_, index) => {
        const [title, description] = SKELETON_WIDTHS[index % SKELETON_WIDTHS.length]!;
        return (
          <div
            aria-hidden="true"
            className="grid min-h-15 grid-cols-[var(--size-provider-logo-well)_minmax(0,1fr)_auto] items-center gap-3 rounded-lg bg-canvas-soft px-3 py-2.5"
            key={index}
          >
            <Skeleton className="size-(--size-provider-logo-well) rounded-md" />
            <div className="flex min-w-0 flex-col gap-1.5">
              <Skeleton className="h-2.75" style={{ width: `${title}%` }} />
              <Skeleton className="h-2.5" style={{ width: `${description}%` }} />
            </div>
            <Skeleton className="h-6 w-13 rounded-md" />
          </div>
        );
      })}
    </MarketplaceGrid>
  );
}

export { MarketplaceGrid, MarketplaceGridSkeleton };
export type { MarketplaceGridProps };
