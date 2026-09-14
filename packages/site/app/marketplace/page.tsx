import { Eyebrow, Pill, PillDot } from "@compozy/ui";
import { ArrowRight } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { MarketplaceBridgeGrid } from "@/components/marketplace/marketplace-bridge-grid";
import { MarketplaceBundledSection } from "@/components/marketplace/marketplace-bundled-section";
import { MarketplaceCatalogSection } from "@/components/marketplace/marketplace-catalog-section";
import { MarketplaceContribute } from "@/components/marketplace/marketplace-contribute";
import { MarketplaceHero } from "@/components/marketplace/marketplace-hero";
import { bridgeProviders } from "@/lib/marketplace-bridges";
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

      {/*
        One catalog leads: everything installable is an extension, so the searchable grid is the
        page. What already ships in the binary follows, then bridge providers, which are extensions
        too but build from source and so read from the in-tree manifests rather than the feed.
      */}
      <div className="mx-auto flex w-full max-w-site-layout-width flex-col gap-14 px-4 pt-14">
        <MarketplaceCatalogSection />

        <MarketplaceBundledSection />

        <section
          aria-labelledby="bridges"
          className="grid gap-6 border-t border-line pt-10 lg:grid-cols-[minmax(0,17rem)_minmax(0,1fr)] lg:gap-10"
        >
          <div>
            <Eyebrow className="text-subtle">Bridge providers</Eyebrow>
            <h2
              id="bridges"
              className="mt-2 flex items-baseline gap-2.5 text-xl font-semibold tracking-[-0.015em] text-fg"
            >
              Bridges
              <span className="font-mono text-small-body font-normal text-subtle">
                {bridgeProviders.length}
              </span>
            </h2>
            <p className="mt-3 text-small-body leading-relaxed text-muted">
              Chat and tracker platforms your agents can live in. Bridges are extensions that
              provide <code>bridge.adapter</code> — grouped by platform, because that is how you
              look for them.
            </p>
            <p className="mt-3 flex flex-wrap items-center gap-2 text-small-body leading-relaxed text-muted">
              <Pill tone="warning" size="sm">
                <PillDot />
                Alpha
              </Pill>
              <span>
                Providers build from source today — <code>extensions/bridges/</code> in the
                repository. There is no packaged install yet.
              </span>
            </p>
            <Link
              href="/marketplace/bridges"
              className="mt-4 inline-flex items-center gap-1.5 text-small-body font-medium text-muted transition-colors hover:text-accent"
            >
              View all bridges
              <ArrowRight aria-hidden className="size-3.5" />
            </Link>
          </div>
          <MarketplaceBridgeGrid providers={bridgeProviders} />
        </section>

        <MarketplaceContribute />
      </div>
    </main>
  );
}
