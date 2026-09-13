import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";

import { StatusDot } from "@compozy/ui";

import type { InstalledExtensionView } from "@/systems/extensions";

import { installedExtensionKey, installedLogoEntry } from "../lib/marketplace-installed-view";
import { MarketplaceEntryLogo } from "./marketplace-entry-logo";

const SHELF_LOGO_LIMIT = 6;

interface MarketplaceInstalledShelfProps {
  items: readonly InstalledExtensionView[];
  updates: number;
  query?: string;
}

/**
 * One 40px line at the top of Browse: up to six installed logos stacked, the count, and the
 * pending updates. The whole line is the one door to the Installed view; at zero it is absent.
 */
function MarketplaceInstalledShelf({ items, updates, query }: MarketplaceInstalledShelfProps) {
  if (items.length === 0) return null;
  const visible = items.slice(0, SHELF_LOGO_LIMIT);
  const overflow = items.length - visible.length;
  const updatesLabel = `${updates} ${updates === 1 ? "update" : "updates"} available`;
  return (
    <Link
      aria-label={`${items.length} installed${updates > 0 ? `, ${updatesLabel}` : ""}. Open Installed`}
      className="-mx-1 flex min-h-10 items-center gap-2.5 rounded-md px-1 text-fg transition-colors duration-fast hover:bg-row-hover hover:text-fg-strong focus-visible:shadow-focus-inset focus-visible:outline-none"
      data-testid="marketplace-installed-shelf"
      search={{ q: query || undefined }}
      to="/marketplace/installed"
    >
      <span aria-hidden="true" className="flex shrink-0 items-center">
        {visible.map(item => (
          <MarketplaceEntryLogo
            className="ring-2 ring-canvas not-first:-ml-1.5"
            entry={installedLogoEntry(item)}
            key={installedExtensionKey(item)}
            size="sm"
          />
        ))}
        {overflow > 0 ? (
          <span className="-ml-1.5 grid size-(--size-catalog-logo) shrink-0 place-items-center rounded-sm bg-elevated font-mono text-mono-id font-semibold text-subtle ring-2 ring-canvas">
            +{overflow}
          </span>
        ) : null}
      </span>
      <span className="inline-flex items-center gap-1.5 text-small-body font-medium whitespace-nowrap">
        <span
          className="font-mono text-mono-id font-semibold text-fg-strong tabular-nums"
          data-testid="marketplace-installed-shelf-count"
        >
          {items.length}
        </span>
        installed
        <ChevronRight aria-hidden="true" className="size-3.5 text-faint" />
      </span>
      {updates > 0 ? (
        <>
          <span aria-hidden="true" className="size-0.5 shrink-0 rounded-full bg-faint" />
          <span
            className="inline-flex items-center gap-1.5 text-eyebrow font-medium whitespace-nowrap text-warning"
            data-testid="marketplace-installed-shelf-updates"
          >
            <StatusDot aria-hidden="true" size="sm" tone="warning" />
            {updatesLabel}
          </span>
        </>
      ) : null}
    </Link>
  );
}

export { MarketplaceInstalledShelf };
export type { MarketplaceInstalledShelfProps };
