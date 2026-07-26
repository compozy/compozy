import { Link } from "@tanstack/react-router";

import { Pill } from "@agh/ui";

import type { MarketplaceEntryResponse, MarketplaceKind } from "../types";
import { formatMarketplaceCount } from "./marketplace-ui";

interface MarketplaceDetailMetaProps {
  entry: MarketplaceEntryResponse["entry"];
}

/**
 * Content metadata for a marketplace detail. Window identity, live status, and
 * the primary action belong to the OS head and are deliberately absent here.
 */
function MarketplaceDetailMeta({ entry }: MarketplaceDetailMetaProps) {
  const kind = entry.kind as MarketplaceKind;
  const blocked = kind === "extension" && entry.trust?.decision === "blocked";

  return (
    <div
      className="flex flex-col gap-2 border-b border-line py-4"
      data-testid="marketplace-detail-meta"
    >
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        <Pill mono>{kind}</Pill>
        {entry.version ? <Pill mono>v{entry.version}</Pill> : null}
      </div>
      <div className="flex flex-wrap items-center gap-2 text-small-body text-muted">
        {entry.author ? <span>{entry.author}</span> : null}
        <span>{entry.source}</span>
        {entry.downloads !== undefined && entry.downloads !== null ? (
          <span>{formatMarketplaceCount(entry.downloads)} downloads</span>
        ) : null}
      </div>
      {blocked ? (
        <p className="text-form-hint text-danger">
          Blocked by extensions policy.
          <Link className="ml-1 underline underline-offset-3" to="/settings/extensions">
            Settings › Extensions
          </Link>
        </p>
      ) : null}
    </div>
  );
}

export { MarketplaceDetailMeta };
export type { MarketplaceDetailMetaProps };
