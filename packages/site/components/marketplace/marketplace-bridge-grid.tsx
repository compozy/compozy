import { Pill, PillDot } from "@compozy/ui";
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
function BridgeMark({ platform }: { platform: string }) {
  return (
    <span
      aria-hidden
      className="inline-flex size-9 shrink-0 items-center justify-center rounded-icon-well border border-line bg-elevated"
    >
      {BRIDGE_LOGOS[platform] ?? <Plug className="size-4 text-muted" />}
    </span>
  );
}

function secretSlotSummary(slots: BridgeProvider["secretSlots"]): string {
  const noun = slots.total === 1 ? "secret slot" : "secret slots";
  if (slots.required === slots.total) {
    return `${slots.total} ${noun}`;
  }
  return `${slots.total} ${noun} · ${slots.required} required`;
}

export function MarketplaceBridgeCard({ provider }: { provider: BridgeProvider }) {
  return (
    <article
      id={provider.platform}
      className="flex scroll-mt-24 flex-col gap-3 rounded-xl border border-line bg-canvas-soft p-4 transition-colors hover:border-line-strong"
    >
      <div className="flex min-w-0 items-center gap-2.5">
        <BridgeMark platform={provider.platform} />
        <div className="min-w-0">
          <p className="truncate text-card-title font-semibold tracking-tight text-fg">
            {provider.displayName}
          </p>
          <span className="font-mono text-badge text-subtle">
            {provider.platform} · v{provider.version}
          </span>
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-2 text-small-body text-muted">
        <Pill tone="warning" size="sm">
          <PillDot />
          Alpha
        </Pill>
        <span>{secretSlotSummary(provider.secretSlots)}</span>
      </div>
      <Link
        href={provider.setupUrl}
        className="inline-flex items-center gap-1.5 border-t border-line pt-3 text-small-body font-medium text-muted transition-colors hover:text-accent"
      >
        Setup guide
        <ArrowRight aria-hidden className="size-3" />
      </Link>
    </article>
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
