import { Check } from "@compozy/ui/icons";
import { useId, type ReactNode } from "react";

import {
  Button,
  Pill,
  Skeleton,
  SkeletonRegion,
  Spinner,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import type { MarketplaceSetupState } from "../lib/marketplace-setup-state";
import type { MarketplaceCatalogListing } from "../types";
import { MarketplaceInstalledServerTrail } from "./marketplace-installed-server-trail";
import {
  MarketplaceInstalledTrailControls,
  type MarketplaceInstalledTrailProps,
} from "./marketplace-installed-trail-controls";
import { formatMarketplaceVersion } from "./marketplace-ui";

const DEFAULT_INSTALL_BLOCKER =
  "Your install policy blocks unverified extensions. Change it under Settings › Extensions.";

/** Installed-row state a Browse row carries; the detail head reads only `installed`. */
interface MarketplaceCatalogTrailSetup {
  state: MarketplaceSetupState;
  /** Cause of the enable an Add could not finish. */
  enableCause?: string | null;
  enabling?: boolean;
  onTurnOn: () => void;
  onRetry: () => void;
  /** The Set up control: it owns its sign-in flow, so the row passes it in. */
  setUpAction?: ReactNode;
}

interface MarketplaceCatalogTrailProps {
  entry: MarketplaceCatalogListing;
  pending?: boolean;
  onInstall: (entry: MarketplaceCatalogListing) => void;
  onUpdate: (entry: MarketplaceCatalogListing) => void;
  setup?: MarketplaceCatalogTrailSetup;
}

/**
 * Catalog trail, one state at a time (top wins): pending → conflict → blocked → update → setup
 * state (Set up · Ready · Turn on · Couldn't turn on · unknown) → installed → Add.
 * Blocked is a status word, never a disabled button pretending to be an action. The same trail
 * rides the catalog row and the detail window head, where actions stay `secondary` — a window
 * head never carries the inverted primary.
 */
function MarketplaceCatalogTrail({
  entry,
  pending = false,
  onInstall,
  onUpdate,
  setup,
}: MarketplaceCatalogTrailProps) {
  const version = formatMarketplaceVersion(entry.version);
  const blocked = entry.trust?.decision === "blocked" || entry.installable === false;

  if (pending) {
    return (
      <Button
        aria-label={`${entry.update_available ? "Updating" : "Adding"} ${entry.name}`}
        data-testid={`marketplace-action-${entry.entry_id}`}
        disabled
        size="sm"
        type="button"
        variant="secondary"
      >
        <Spinner aria-hidden="true" className="size-3.5" />
        {entry.update_available ? "Updating…" : "Adding…"}
      </Button>
    );
  }

  if (entry.name_conflict) {
    const origin = entry.name_conflict;
    return (
      <Pill
        className="cursor-help"
        data-testid={`marketplace-name-conflict-${entry.entry_id}`}
        form="plain"
        title={`This name is already installed from ${origin.source}/${origin.entry_id} (${origin.source_ref}).`}
        tone="warning"
      >
        <Pill.Dot />
        Name in use
      </Pill>
    );
  }

  if (blocked) {
    return (
      <Pill
        className="cursor-help"
        data-testid={`marketplace-blocked-${entry.entry_id}`}
        form="plain"
        title={entry.install_blocker?.trim() || DEFAULT_INSTALL_BLOCKER}
        tone="destructive"
      >
        <Pill.Dot />
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
          variant="secondary"
        >
          Update
        </Button>
      </>
    );
  }

  if (entry.installed && setup) {
    const word = <MarketplaceSetupWord entry={entry} setup={setup} />;
    if (word) return word;
  }

  if (entry.installed) {
    return (
      <span
        className="inline-flex items-center gap-1.5 text-eyebrow font-medium text-muted-foreground"
        data-testid={`marketplace-installed-${entry.entry_id}`}
      >
        <Check aria-hidden="true" className="size-3" />
        Installed
      </span>
    );
  }

  return (
    <Button
      aria-label={`Add ${entry.name}`}
      data-testid={`marketplace-action-${entry.entry_id}`}
      onClick={() => onInstall(entry)}
      size="sm"
      type="button"
      variant="secondary"
    >
      Add
    </Button>
  );
}

/** The installed half of US-007: one word or one button, never Ready without inventory truth. */
function MarketplaceSetupWord({
  entry,
  setup,
}: {
  entry: MarketplaceCatalogListing;
  setup: MarketplaceCatalogTrailSetup;
}): ReactNode {
  switch (setup.state) {
    case "unknown":
      return (
        <SkeletonRegion
          data-testid={`marketplace-setup-unknown-${entry.entry_id}`}
          label={`Checking ${entry.name}`}
        >
          <Skeleton className="h-button-sm w-16" />
        </SkeletonRegion>
      );
    case "setup":
      return setup.setUpAction ?? null;
    case "ready":
      return (
        <span
          className="inline-flex items-center gap-1.5 text-eyebrow font-medium text-subtle"
          data-testid={`marketplace-ready-${entry.entry_id}`}
        >
          <Check aria-hidden="true" className="size-3" />
          Ready
        </span>
      );
    case "turn-on":
      return (
        <Button
          aria-label={`Turn on ${entry.name}`}
          data-testid={`marketplace-turn-on-${entry.entry_id}`}
          disabled={setup.enabling}
          onClick={setup.onTurnOn}
          size="sm"
          type="button"
          variant="ghost"
        >
          {setup.enabling ? <Spinner aria-hidden="true" className="size-3.5" /> : null}
          Turn on
        </Button>
      );
    case "enable-failed":
      return <MarketplaceEnableFailed entry={entry} setup={setup} />;
    default:
      return null;
  }
}

/**
 * Couldn't turn on + Retry. The cause is the Retry button's accessible description and its
 * tooltip, so keyboard, touch and screen-reader users reach it, not only a mouse hover.
 */
function MarketplaceEnableFailed({
  entry,
  setup,
}: {
  entry: MarketplaceCatalogListing;
  setup: MarketplaceCatalogTrailSetup;
}) {
  const causeId = useId();
  const cause = setup.enableCause ? `Added but couldn't turn on: ${setup.enableCause}` : null;
  const retry = (
    <Button
      aria-describedby={cause ? causeId : undefined}
      aria-label={`Retry turning on ${entry.name}`}
      data-testid={`marketplace-enable-retry-${entry.entry_id}`}
      disabled={setup.enabling}
      onClick={setup.onRetry}
      size="sm"
      type="button"
      variant="ghost"
    >
      {setup.enabling ? <Spinner aria-hidden="true" className="size-3.5" /> : null}
      Retry
    </Button>
  );
  return (
    <>
      <Pill
        data-testid={`marketplace-enable-failed-${entry.entry_id}`}
        form="plain"
        tone="destructive"
      >
        <Pill.Dot />
        Couldn't turn on
      </Pill>
      {cause ? (
        <>
          <span className="sr-only" id={causeId}>
            {cause}
          </span>
          <Tooltip>
            <TooltipTrigger render={retry} />
            <TooltipContent>{cause}</TooltipContent>
          </Tooltip>
        </>
      ) : (
        retry
      )}
    </>
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
export type {
  MarketplaceCatalogTrailProps,
  MarketplaceCatalogTrailSetup,
  MarketplaceInstalledTrailProps,
};
