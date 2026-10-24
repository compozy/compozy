import { FileText, Package, PackageX } from "lucide-react";

import {
  Button,
  cn,
  Empty,
  ListingRowSkeleton,
  PAGE_CONTENT_GUTTER,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
} from "@compozy/ui";

import type { MarketplaceCatalogEntryResponse } from "../types";
import { MarketplaceDetailExtensionView } from "./marketplace-detail-extension";
import { MarketplaceDetailLede } from "./marketplace-detail-lede";
import {
  MarketplaceDetailColumns,
  MarketplaceDetailRailCardSkeleton,
  MarketplaceDetailSectionSkeleton,
} from "./marketplace-detail-shell";

interface MarketplaceDetailProps {
  data: MarketplaceCatalogEntryResponse;
  liveDataEnabled?: boolean;
}

/**
 * Marketplace entry detail: the lede carries identity, the extension body fills the page —
 * provenance and trust when browsing, the kit and its runtime once installed. Window identity
 * and the primary action live in the OS head.
 */
function MarketplaceDetail({ data, liveDataEnabled = true }: MarketplaceDetailProps) {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto" data-testid="marketplace-detail">
      <div className={cn(PAGE_CONTENT_GUTTER, "flex flex-col pt-6 pb-20")}>
        <MarketplaceDetailLede data={data} />
        <MarketplaceDetailExtensionView data={data} liveDataEnabled={liveDataEnabled} />
      </div>
    </div>
  );
}

/**
 * The detail page while the entry loads: the real scroller, gutter and two-column grid, the lede
 * at its large-logo layout, a section of placeholder rows and the About rail card.
 */
function MarketplaceDetailSkeleton() {
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

function MarketplaceDetailNotFound({ onBack }: { onBack: () => void }) {
  return (
    <div className={cn(PAGE_CONTENT_GUTTER, "flex flex-col pb-20")}>
      <Empty
        fill={false}
        className="py-14"
        titleAs="h2"
        icon={PackageX}
        title="This item is no longer in the marketplace"
        description="The catalog source no longer returns this entry."
        action={
          <Button onClick={onBack} size="sm" type="button" variant="neutral">
            Back to marketplace
          </Button>
        }
      />
    </div>
  );
}

export { MarketplaceDetail, MarketplaceDetailNotFound, MarketplaceDetailSkeleton };
export type { MarketplaceDetailProps };
