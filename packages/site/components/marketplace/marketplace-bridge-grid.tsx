import { CatalogCard, Pill, PillDot } from "@compozy/ui";
import { ArrowRight, Plug } from "lucide-react";
import Link from "next/link";
import { BRIDGE_LOGOS } from "@/lib/marketplace-bridge-logos";
import type { BridgeProvider } from "@/lib/marketplace-bridges";

/**
 * Tiles use the real platform marks from the `@compozy/ui` logo inventory — the same components the
 * landing page's bridges section renders. A platform with no mark in the inventory falls back to a
 * neutral glyph rather than an invented one. The Alpha pill is the claim — providers build from
 * source today — and the only action is the setup guide, because there is no packaged install.
 */
function secretSlotSummary(slots: BridgeProvider["secretSlots"]): string {
  const noun = slots.total === 1 ? "secret slot" : "secret slots";
  if (slots.required === slots.total) {
    return `${slots.total} ${noun}`;
  }
  return `${slots.total} ${noun} · ${slots.required} required`;
}

function MarketplaceBridgeCard({ provider }: { provider: BridgeProvider }) {
  return (
    <CatalogCard id={provider.platform} actionable className="scroll-mt-24 border border-line">
      <div className="flex min-w-0 items-center gap-2.5">
        <CatalogCard.Logo tone="neutral" size="lg" className="border border-line bg-elevated">
          {BRIDGE_LOGOS[provider.platform] ?? <Plug className="size-4 text-muted" />}
        </CatalogCard.Logo>
        <div className="min-w-0">
          <CatalogCard.Title className="text-card-title font-semibold tracking-tight text-fg">
            {provider.displayName}
          </CatalogCard.Title>
          <span className="font-mono text-badge text-subtle">
            {provider.platform} · v{provider.version}
          </span>
        </div>
      </div>
      <CatalogCard.Meta className="flex flex-wrap items-center gap-2 text-small-body text-muted">
        <Pill tone="warning" size="sm">
          <PillDot />
          Alpha
        </Pill>
        <span>{secretSlotSummary(provider.secretSlots)}</span>
      </CatalogCard.Meta>
      <CatalogCard.Actions>
        <Link
          href={provider.setupUrl}
          className="inline-flex items-center gap-1.5 text-small-body font-medium text-muted transition-colors duration-base ease-out hover:text-accent"
        >
          Setup guide
          <ArrowRight aria-hidden className="size-3" />
        </Link>
      </CatalogCard.Actions>
    </CatalogCard>
  );
}

export function MarketplaceBridgeGrid({ providers }: { providers: BridgeProvider[] }) {
  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      {providers.map(provider => (
        <MarketplaceBridgeCard key={provider.platform} provider={provider} />
      ))}
    </div>
  );
}
