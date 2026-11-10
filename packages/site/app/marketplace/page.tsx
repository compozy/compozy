import type { Metadata } from "next";
import { Suspense } from "react";

import { FromTheApp, Publish, SourcesAndIncluded } from "@/components/marketplace/closing";
import {
  MarketplaceBrowse,
  MarketplaceBrowseFromUrl,
} from "@/components/marketplace/marketplace-browse";
import { MARKETPLACE_DESCRIPTION } from "@/lib/marketplace-copy";
import { marketplaceCards } from "@/lib/marketplace-view";
import { createPageMetadata } from "@/lib/site-config";

export const metadata: Metadata = createPageMetadata({
  title: "Marketplace",
  description: MARKETPLACE_DESCRIPTION,
  path: "/marketplace",
});

export default function MarketplacePage() {
  return (
    <>
      {/* The prerendered page starts on All; the URL's `?category=` applies once it hydrates. */}
      <Suspense fallback={<MarketplaceBrowse entries={marketplaceCards} initialCategory={null} />}>
        <MarketplaceBrowseFromUrl entries={marketplaceCards} />
      </Suspense>
      <FromTheApp />
      <SourcesAndIncluded />
      <Publish />
    </>
  );
}
