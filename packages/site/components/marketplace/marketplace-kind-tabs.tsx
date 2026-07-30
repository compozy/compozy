import Link from "next/link";
import { bridgeProviders } from "@/lib/marketplace-bridges";
import { entriesForKind, type MarketplaceKind } from "@/lib/marketplace-catalog";
import { MARKETPLACE_KIND_META } from "./marketplace-kind-meta";

/**
 * Links, not client-side tab panes.
 *
 * The reference prototype switches panes on one `/marketplace/all` page with `#hash` deep links.
 * Keeping a route per kind preserves per-kind metadata, static generation, and shareable URLs while
 * reading identically — the strip is the same strip, and `aria-current` marks the active one.
 */

export type MarketplaceTabKey = MarketplaceKind | "bridges";

function tabs(): Array<{ key: MarketplaceTabKey; label: string; href: string; count: number }> {
  const kindTabs = MARKETPLACE_KIND_META.map(meta => ({
    key: meta.kind,
    label: meta.title,
    href: `/marketplace/${meta.kind}`,
    count: entriesForKind(meta.kind).length,
  }));

  return [
    ...kindTabs,
    {
      key: "bridges",
      label: "Bridges",
      href: "/marketplace/bridges",
      count: bridgeProviders.length,
    },
  ];
}

export function MarketplaceKindTabs({ active }: { active: MarketplaceTabKey }) {
  return (
    <nav aria-label="Catalog kind" className="flex flex-wrap gap-0.5">
      {tabs().map(tab => {
        const current = tab.key === active;
        return (
          <Link
            key={tab.key}
            href={tab.href}
            aria-current={current ? "page" : undefined}
            className={
              current
                ? "-mb-px inline-flex h-10 items-center gap-1.5 border-b-2 border-fg px-3 text-sm font-medium text-fg"
                : "-mb-px inline-flex h-10 items-center gap-1.5 border-b-2 border-transparent px-3 text-sm font-medium text-muted transition-colors hover:text-fg"
            }
          >
            {tab.label}
            <span className="rounded-full bg-neutral-tint px-1.5 py-px font-mono text-micro text-subtle">
              {tab.count}
            </span>
          </Link>
        );
      })}
    </nav>
  );
}
