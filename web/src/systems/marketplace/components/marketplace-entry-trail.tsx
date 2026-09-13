import { Check } from "lucide-react";

import { Button, Pill, Spinner } from "@compozy/ui";

import type { MarketplaceCatalogListing } from "../types";
import { MarketplaceInstalledServerTrail } from "./marketplace-installed-server-trail";
import {
  MarketplaceInstalledTrailControls,
  type MarketplaceInstalledTrailProps,
} from "./marketplace-installed-trail-controls";
import { formatMarketplaceVersion } from "./marketplace-ui";

const DEFAULT_INSTALL_BLOCKER =
  "Your install policy blocks unverified extensions. Change it under Settings › Extensions.";

interface MarketplaceCatalogTrailProps {
  entry: MarketplaceCatalogListing;
  pending?: boolean;
  /** `primary` only in the detail head, where the one accent lives. */
  emphasis?: "neutral" | "primary";
  onInstall: (entry: MarketplaceCatalogListing) => void;
  onUpdate: (entry: MarketplaceCatalogListing) => void;
}

/**
 * Catalog trail, one state at a time (top wins): pending → blocked → update → installed → install.
 * Blocked is a sentence, never a disabled button pretending to be an action.
 */
function MarketplaceCatalogTrail({
  entry,
  pending = false,
  emphasis = "neutral",
  onInstall,
  onUpdate,
}: MarketplaceCatalogTrailProps) {
  const version = formatMarketplaceVersion(entry.version);
  const blocked = entry.trust?.decision === "blocked" || entry.installable === false;
  const variant = emphasis === "primary" ? "default" : "neutral";

  if (pending) {
    return (
      <Button
        aria-label={`${entry.update_available ? "Updating" : "Installing"} ${entry.name}`}
        data-testid={`marketplace-action-${entry.entry_id}`}
        disabled
        size="sm"
        type="button"
        variant={variant}
      >
        <Spinner aria-hidden="true" className="size-3" />
        {entry.update_available ? "Updating…" : "Installing…"}
      </Button>
    );
  }

  if (entry.name_conflict) {
    const origin = entry.name_conflict;
    return (
      <Pill
        className="cursor-help"
        data-testid={`marketplace-name-conflict-${entry.entry_id}`}
        form="hollow"
        size="xs"
        title={`This name is already installed from ${origin.source}/${origin.entry_id} (${origin.source_ref}).`}
        tone="warning"
      >
        Name in use
      </Pill>
    );
  }

  if (blocked) {
    return (
      <Pill
        className="cursor-help"
        data-testid={`marketplace-blocked-${entry.entry_id}`}
        form="hollow"
        size="xs"
        title={entry.install_blocker?.trim() || DEFAULT_INSTALL_BLOCKER}
        tone="danger"
      >
        Blocked
      </Pill>
    );
  }

  if (entry.update_available) {
    return (
      <>
        {version ? (
          <span className="font-mono text-mono-id text-faint tabular-nums">{version}</span>
        ) : null}
        <Button
          aria-label={`Update ${entry.name}`}
          data-testid={`marketplace-action-${entry.entry_id}`}
          onClick={() => onUpdate(entry)}
          size="sm"
          type="button"
          variant={variant}
        >
          Update
        </Button>
      </>
    );
  }

  if (entry.installed) {
    return (
      <span
        className="inline-flex items-center gap-1.5 text-eyebrow font-medium text-success"
        data-testid={`marketplace-installed-${entry.entry_id}`}
      >
        <Check aria-hidden="true" className="size-3" />
        Installed
      </span>
    );
  }

  return (
    <Button
      aria-label={`Install ${entry.name}`}
      data-testid={`marketplace-action-${entry.entry_id}`}
      onClick={() => onInstall(entry)}
      size="sm"
      type="button"
      variant={variant}
    >
      Install
    </Button>
  );
}

/**
 * Installed trail: the readiness word first, then update, the enable switch and the overflow. A
 * dev overlay shadows the published row without owning it, so enable/update stay on the published
 * extension. "Needs configuration" is daemon truth (`missing_inputs`), never inferred from the
 * listing. Rows whose extension provides an MCP server add the server's status word, Authorize
 * and Edit configuration on top of the same controls.
 */
function MarketplaceInstalledTrail(props: MarketplaceInstalledTrailProps) {
  if (props.item.extension.mcp_servers.length > 0) {
    return <MarketplaceInstalledServerTrail {...props} />;
  }
  return <MarketplaceInstalledTrailControls {...props} />;
}

export { MarketplaceCatalogTrail, MarketplaceInstalledTrail };
export type { MarketplaceCatalogTrailProps, MarketplaceInstalledTrailProps };
