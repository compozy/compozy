import { FileText, Package } from "lucide-react";

import {
  cn,
  ListingRowSkeleton,
  PAGE_CONTENT_GUTTER,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
} from "@compozy/ui";

import {
  MarketplaceDetailColumns,
  MarketplaceDetailRailCardSkeleton,
  MarketplaceDetailSectionSkeleton,
} from "./marketplace-detail-shell";

/**
 * The detail page while the entry loads: the real scroller, gutter and two-column grid, the lede
 * at its large-logo layout, a section of placeholder rows and the About rail card.
 */
export function MarketplaceDetailSkeleton() {
  return (
    <div
      className="flex min-h-0 flex-1 flex-col overflow-y-auto"
      data-testid="marketplace-detail-loading"
    >
      <SkeletonRegion
        className={cn(PAGE_CONTENT_GUTTER, "flex flex-col pt-6 pb-20")}
        label="Loading marketplace entry"
      >
        <header
          aria-hidden="true"
          className="mb-6 flex items-start gap-3.5 border-b border-line pb-5"
        >
          <Skeleton className="size-14 shrink-0 rounded-lg" />
          <div className="min-w-0 flex-1">
            <div className="text-detail-h1">
              <SkeletonText className="w-56" />
            </div>
            <div className="mt-1.5 text-form-label">
              <SkeletonText className="w-72" />
            </div>
            <div className="mt-2.5 max-w-prose text-small-body leading-relaxed">
              <SkeletonText lines={2} />
            </div>
          </div>
        </header>
        <MarketplaceDetailColumns
          main={
            <MarketplaceDetailSectionSkeleton icon={Package}>
              <ListingRowSkeleton count={3} trail="pill" />
            </MarketplaceDetailSectionSkeleton>
          }
          rail={<MarketplaceDetailRailCardSkeleton icon={FileText} rows={5} title="About" />}
        />
      </SkeletonRegion>
    </div>
  );
}
