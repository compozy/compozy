import { PackageX } from "@compozy/ui/icons";

import { Button, cn, Empty, PAGE_CONTENT_GUTTER } from "@compozy/ui";

import type { MarketplaceCatalogEntryResponse } from "../types";
import { MarketplaceDetailExtensionView } from "./marketplace-detail-extension";
import { MarketplaceDetailLede } from "./marketplace-detail-lede";

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

export { MarketplaceDetail, MarketplaceDetailNotFound };
export type { MarketplaceDetailProps };
