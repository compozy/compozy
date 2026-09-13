import { Link } from "@tanstack/react-router";
import { Fragment, type ReactNode } from "react";

import { Pill, Time } from "@compozy/ui";

import { ExtensionFormatBadge } from "@/systems/extensions";

import type { MarketplaceCatalogEntryResponse } from "../types";
import { MarketplaceEntryLogo } from "./marketplace-entry-logo";
import { formatMarketplaceVersion } from "./marketplace-ui";

interface MarketplaceDetailLedeProps {
  data: MarketplaceCatalogEntryResponse;
}

/**
 * Page lede: the entry logo at its large rung, the name with version and format tags, one
 * metadata line, and the one-sentence description. Window identity and the primary action stay
 * in the OS head.
 */
function MarketplaceDetailLede({ data }: MarketplaceDetailLedeProps) {
  const entry = data.entry;
  const blocked = entry.trust?.decision === "blocked";
  const description = entry.description?.trim();
  const version = formatMarketplaceVersion(entry.version);
  const meta = ledeMeta(data);

  return (
    <header
      className="mb-6 flex items-start gap-3.5 border-b border-line pb-5"
      data-testid="marketplace-detail-lede"
    >
      <MarketplaceEntryLogo entry={entry} size="lg" />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h1 className="text-detail-h1 leading-tight font-semibold tracking-detail-h1 text-fg-strong">
            {entry.name}
          </h1>
          {version ? (
            <Pill mono size="xs">
              {version}
            </Pill>
          ) : null}
          {entry.trust?.registry_tier === "unverified" ? (
            <Pill form="hollow" size="xs" tone="warning">
              unverified
            </Pill>
          ) : null}
          <ExtensionFormatBadge format={entry.format} />
        </div>
        {meta.length > 0 ? (
          <div className="mt-1.5 flex min-w-0 flex-wrap items-center gap-2 text-form-label text-subtle">
            {meta.map((item, index) => (
              <Fragment key={item.key}>
                {index > 0 ? (
                  <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
                ) : null}
                {item.node}
              </Fragment>
            ))}
          </div>
        ) : null}
        {description ? (
          <p className="mt-2.5 max-w-[70ch] text-small-body leading-relaxed text-muted">
            {description}
          </p>
        ) : null}
        {blocked ? (
          <p className="mt-2 text-form-hint text-danger">
            Blocked by extensions policy.
            <Link className="ml-1 underline underline-offset-3" to="/settings/extensions">
              Settings › Extensions
            </Link>
          </p>
        ) : null}
      </div>
    </header>
  );
}

interface LedeMetaItem {
  key: string;
  node: ReactNode;
}

function ledeMeta(data: MarketplaceCatalogEntryResponse): LedeMetaItem[] {
  const entry = data.entry;
  const items: LedeMetaItem[] = [];
  if (entry.author) items.push({ key: "author", node: <span>{entry.author}</span> });
  items.push({ key: "source", node: <span>{entry.source}</span> });

  if (entry.tier) items.push({ key: "tier", node: <span>{entry.tier} tier</span> });
  if (entry.updated_at) {
    items.push({
      key: "updated",
      node: (
        <span>
          updated <Time iso={entry.updated_at} />
        </span>
      ),
    });
  }
  return items;
}

export { MarketplaceDetailLede };
export type { MarketplaceDetailLedeProps };
