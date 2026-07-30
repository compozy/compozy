import { Eyebrow, Pill, PillDot } from "@compozy/ui";
import type { Metadata } from "next";
import { MarketplaceBridgeGrid } from "@/components/marketplace/marketplace-bridge-grid";
import { MarketplaceCrumbs } from "@/components/marketplace/marketplace-crumbs";
import { MarketplaceKindTabs } from "@/components/marketplace/marketplace-kind-tabs";
import { bridgeProviders } from "@/lib/marketplace-bridges";
import { marketplaceBridgesDescription } from "@/lib/marketplace-copy";
import { createPageMetadata } from "@/lib/site-config";

/**
 * A static segment, so it wins over `[kind]` — bridges are deliberately not a catalog kind. The feed
 * kinds stay capped at three (`internal/marketplace/types.go`), and this page reads the in-repo
 * provider manifests instead.
 */
export const metadata: Metadata = createPageMetadata({
  title: "Bridges — Marketplace",
  description: marketplaceBridgesDescription(bridgeProviders.length),
  path: "/marketplace/bridges",
});

const firstBridgeProvider = bridgeProviders[0];
if (!firstBridgeProvider) {
  throw new Error("Marketplace bridges require at least one in-tree provider manifest");
}

export default function MarketplaceBridgesPage() {
  return (
    <main id="main-content" className="w-full pb-20">
      <header className="border-b border-line bg-canvas-soft">
        <div className="mx-auto w-full max-w-site-layout-width px-4 pt-10 pb-9">
          <MarketplaceCrumbs leaf="Bridges" />
          <h1 className="mt-4 font-display text-site-doc-heading leading-tight font-normal tracking-[-0.02em] text-fg">
            Bridge providers
          </h1>
          <p className="mt-3.5 max-w-[64ch] text-site-lead text-muted">
            Chat and tracker platforms your agents can live in. Each provider is an extension that
            supplies <code>bridge.adapter</code>, and each one has an operator setup guide.
          </p>
          <p className="mt-5 flex max-w-[68ch] flex-wrap items-center gap-2 text-small-body leading-relaxed text-muted">
            <Pill tone="warning" size="sm">
              <PillDot />
              Alpha
            </Pill>
            <span>
              Providers build from source today. Released <code>compozy</code> artifacts do not
              include these executables, and packaged installs land with the catalog feeds.
            </span>
          </p>
        </div>
      </header>

      <div className="mx-auto w-full max-w-site-layout-width px-4 pt-11">
        <div className="flex flex-wrap items-end border-b border-line">
          <MarketplaceKindTabs active="bridges" />
        </div>

        <div className="mt-7">
          <MarketplaceBridgeGrid providers={bridgeProviders} />
        </div>

        <section aria-labelledby="build-from-source" className="mt-14 border-t border-line pt-10">
          <Eyebrow className="text-subtle">Install</Eyebrow>
          <h2
            id="build-from-source"
            className="mt-2 text-xl font-semibold tracking-[-0.015em] text-fg"
          >
            Build a provider from source
          </h2>
          <p className="mt-3 max-w-[68ch] text-small-body leading-relaxed text-muted">
            Run this from the root of a trusted Compozy source checkout with the daemon running. The
            local install is an explicit trust decision: it copies the provider into your extension
            home and enables it.
          </p>
          <div className="mt-5 overflow-x-auto rounded-lg border border-line bg-canvas p-4">
            <pre className="m-0 font-mono text-small-body leading-relaxed text-fg">
              {`PROVIDER=${firstBridgeProvider.platform}
mkdir -p "./extensions/bridges/$PROVIDER/bin"
go build -o "./extensions/bridges/$PROVIDER/bin/$PROVIDER" "./extensions/bridges/$PROVIDER"
compozy extension install "./extensions/bridges/$PROVIDER" --allow-unverified --yes -o json
compozy extension status "$PROVIDER" -o json`}
            </pre>
          </div>
          <p className="mt-4 text-small-body leading-relaxed text-subtle">
            Public console steps, credential slots, and recovery paths live in each provider's setup
            guide above.
          </p>
        </section>
      </div>
    </main>
  );
}
