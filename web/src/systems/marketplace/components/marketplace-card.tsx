import { Link } from "@tanstack/react-router";
import { createElement } from "react";

import { CatalogCard } from "@agh/ui";

import type { MarketplaceKind, MarketplaceListing } from "../types";
import { MarketplaceEntryAction, MarketplaceEntryStatus } from "./marketplace-entry-actions";
import { formatMarketplaceCount, marketplaceKindIcon } from "./marketplace-ui";

interface MarketplaceCardProps {
  entry: MarketplaceListing;
  pending?: boolean;
  flashing?: boolean;
  onAction: (entry: MarketplaceListing) => void;
  onFlashEnd?: (entry: MarketplaceListing) => void;
}

function MarketplaceCard({
  entry,
  pending = false,
  flashing = false,
  onAction,
  onFlashEnd,
}: MarketplaceCardProps) {
  const kind = entry.kind as MarketplaceKind;
  return (
    <CatalogCard
      actionable={!pending}
      className={flashing ? "marketplace-card-flash" : pending ? "opacity-55" : undefined}
      data-testid={`marketplace-card-${entry.entry_id}`}
      onAnimationEnd={event => {
        if (event.currentTarget === event.target) onFlashEnd?.(entry);
      }}
    >
      <Link
        aria-disabled={pending || undefined}
        aria-label={`View ${entry.name} details`}
        className="flex min-w-0 flex-col gap-3 rounded focus-visible:outline-none focus-visible:shadow-focus-ring"
        onClick={event => {
          if (pending) event.preventDefault();
        }}
        params={{ entryId: entry.entry_id, kind }}
        tabIndex={pending ? -1 : undefined}
        to="/marketplace/$kind/$entryId"
      >
        <div className="flex min-w-0 items-start gap-3">
          <CatalogCard.Logo tone={kind === "extension" ? "neutral" : "accent"}>
            {createElement(marketplaceKindIcon(kind), {
              "aria-hidden": true,
              className: "size-3.5",
            })}
          </CatalogCard.Logo>
          <div className="min-w-0 flex-1">
            <CatalogCard.Title>{entry.name}</CatalogCard.Title>
            <CatalogCard.Meta>
              {entry.author ? <span>{entry.author}</span> : null}
              {entry.version ? (
                <span className="normal-case font-mono tracking-normal">{`v${entry.version}`}</span>
              ) : null}
              {entry.downloads !== undefined && entry.downloads !== null ? (
                <span className="normal-case tabular-nums">
                  {formatMarketplaceCount(entry.downloads)}
                </span>
              ) : null}
              {entry.transport ? (
                <span className="normal-case font-mono tracking-normal">{entry.transport}</span>
              ) : null}
              {entry.tier ? <span>{entry.tier}</span> : null}
            </CatalogCard.Meta>
          </div>
        </div>
        <CatalogCard.Description className="line-clamp-2 min-h-12">
          {entry.description}
        </CatalogCard.Description>
      </Link>

      <CatalogCard.Actions>
        <MarketplaceEntryStatus entry={entry} />
        <div className="ml-auto">
          <MarketplaceEntryAction entry={entry} onAction={onAction} pending={pending} />
        </div>
      </CatalogCard.Actions>
    </CatalogCard>
  );
}

export { MarketplaceCard };
export type { MarketplaceCardProps };
