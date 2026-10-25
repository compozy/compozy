import type { ReactNode } from "react";

import {
  CatalogCard,
  CatalogCardSkeleton,
  cn,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
} from "@compozy/ui";

import { MARKETPLACE_ENTRY_CARD_CLASS } from "./marketplace-classes";

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

const SKELETON_CARDS = [
  { id: "skeleton-card-1", title: "w-40", description: "w-3/4" },
  { id: "skeleton-card-2", title: "w-32", description: "w-2/3" },
  { id: "skeleton-card-3", title: "w-48", description: "w-4/5" },
  { id: "skeleton-card-4", title: "w-36", description: "w-3/5" },
  { id: "skeleton-card-5", title: "w-44", description: "w-2/3" },
  { id: "skeleton-card-6", title: "w-28", description: "w-3/4" },
  { id: "skeleton-card-7", title: "w-52", description: "w-3/5" },
  { id: "skeleton-card-8", title: "w-36", description: "w-4/5" },
] as const;

interface MarketplaceGridSkeletonProps {
  count?: number;
  className?: string;
}

/**
 * Placeholder row-cards in the real grid. Each is the real `CatalogCard` surface with the entry
 * card's own layout class, so a card keeps its height and column grid when the catalog arrives.
 * The caller owns the status region (a page that loads more than the grid wraps all of it in one).
 */
function MarketplaceGridPlaceholder({ count = 6, className }: MarketplaceGridSkeletonProps) {
  return (
    <MarketplaceGrid className={className} data-testid="marketplace-grid-skeleton">
      {SKELETON_CARDS.slice(0, count).map(card => (
        <CatalogCardSkeleton className={MARKETPLACE_ENTRY_CARD_CLASS} key={card.id}>
          <Skeleton className="size-(--size-provider-logo-well) rounded-md" />
          <div className="flex min-w-0 flex-col gap-0.5">
            <CatalogCard.Title>
              <SkeletonText className={card.title} />
            </CatalogCard.Title>
            <CatalogCard.Description className="text-eyebrow leading-snug">
              <SkeletonText className={card.description} />
            </CatalogCard.Description>
          </div>
          <Skeleton className="h-button-sm w-16" />
        </CatalogCardSkeleton>
      ))}
    </MarketplaceGrid>
  );
}

/** The placeholder grid under its own status region. */
function MarketplaceGridSkeleton(props: MarketplaceGridSkeletonProps) {
  return (
    <SkeletonRegion label="Loading marketplace entries">
      <MarketplaceGridPlaceholder {...props} />
    </SkeletonRegion>
  );
}

export { MarketplaceGrid, MarketplaceGridPlaceholder, MarketplaceGridSkeleton };
export type { MarketplaceGridProps };
