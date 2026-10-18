import type { Metadata } from "next";
import { MarketplaceBundledSection } from "@/components/marketplace/marketplace-bundled-section";
import { MarketplaceCatalogSection } from "@/components/marketplace/marketplace-catalog-section";
import { MarketplaceContribute } from "@/components/marketplace/marketplace-contribute";
import { MarketplaceHero } from "@/components/marketplace/marketplace-hero";
import { MARKETPLACE_DESCRIPTION } from "@/lib/marketplace-copy";
import { createPageMetadata } from "@/lib/site-config";

export const metadata: Metadata = createPageMetadata({
  title: "Marketplace",
  description: MARKETPLACE_DESCRIPTION,
  path: "/marketplace",
});

export default function MarketplacePage() {
  return (
    <main id="main-content" className="w-full pb-20">
      <MarketplaceHero />

      <div className="mx-auto flex w-full max-w-site-layout-width flex-col gap-14 px-4 pt-14">
        <MarketplaceCatalogSection />

        <MarketplaceBundledSection />

        <MarketplaceContribute />
      </div>
    </main>
  );
}
