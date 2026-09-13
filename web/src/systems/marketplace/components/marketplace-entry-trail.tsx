import { Link } from "@tanstack/react-router";
import { Check, MoreHorizontal } from "lucide-react";
import { useState } from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Pill,
  Spinner,
  Switch,
} from "@compozy/ui";

import { RemoveExtensionDialog, type InstalledExtensionView } from "@/systems/extensions";

import {
  installedDetailSearch,
  installedDisplayName,
  installedEntryId,
} from "../lib/marketplace-installed-view";
import type { MarketplaceCatalogListing } from "../types";
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

interface MarketplaceInstalledTrailProps {
  item: InstalledExtensionView;
  pending?: boolean;
  /** Search kept on the detail link so Back returns here with the query intact. */
  query?: string;
  onUpdate: (item: InstalledExtensionView) => void;
  onToggleEnabled: (item: InstalledExtensionView, enabled: boolean) => void;
}

/**
 * Installed trail: update first, then the enable switch and the overflow. A dev overlay shadows
 * the published row without owning it, so enable/update stay on the published extension.
 */
function MarketplaceInstalledTrail({
  item,
  pending = false,
  query,
  onUpdate,
  onToggleEnabled,
}: MarketplaceInstalledTrailProps) {
  const [removing, setRemoving] = useState(false);
  const { extension } = item;
  const name = installedDisplayName(item);
  const isDevOverlay = extension.dev === true;
  const targetVersion = formatMarketplaceVersion(item.listing?.version ?? extension.remote_version);
  const detailLink = {
    params: { entryId: installedEntryId(item) },
    search: { ...installedDetailSearch(item), from: "installed" as const, q: query || undefined },
    to: "/marketplace/$entryId" as const,
  };

  return (
    <>
      {item.updateAvailable && !isDevOverlay ? (
        <>
          {targetVersion ? (
            <span className="font-mono text-mono-id text-faint tabular-nums">{targetVersion}</span>
          ) : null}
          <Button
            aria-label={`Update ${name}`}
            data-testid={`marketplace-installed-update-${extension.name}`}
            disabled={pending}
            onClick={() => onUpdate(item)}
            size="sm"
            type="button"
            variant="neutral"
          >
            {pending ? <Spinner aria-hidden="true" className="size-3" /> : null}
            {pending ? "Updating…" : "Update"}
          </Button>
        </>
      ) : null}
      <Switch
        aria-label={
          isDevOverlay ? `Enable ${name} · managed on the published extension` : `Enable ${name}`
        }
        checked={extension.enabled}
        data-testid={`marketplace-installed-switch-${extension.name}`}
        disabled={pending || isDevOverlay}
        onCheckedChange={checked => onToggleEnabled(item, checked)}
        size="sm"
      />
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              aria-haspopup="menu"
              aria-label={`More for ${name}`}
              data-testid={`marketplace-installed-more-${extension.name}`}
              disabled={pending}
              size="icon-sm"
              type="button"
              variant="ghost"
            />
          }
        >
          <MoreHorizontal aria-hidden="true" className="size-3.5" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem render={<Link {...detailLink} />}>View details</DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            className="text-danger"
            data-testid={`marketplace-installed-remove-${extension.name}`}
            onClick={() => setRemoving(true)}
          >
            {isDevOverlay ? "Unlink dev overlay…" : "Remove…"}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <RemoveExtensionDialog extension={extension} onOpenChange={setRemoving} open={removing} />
    </>
  );
}

export { MarketplaceCatalogTrail, MarketplaceInstalledTrail };
export type { MarketplaceCatalogTrailProps, MarketplaceInstalledTrailProps };
