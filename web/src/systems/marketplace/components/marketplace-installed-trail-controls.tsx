import { Link } from "@tanstack/react-router";
import { MoreHorizontal } from "lucide-react";
import { useState, type ReactNode } from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Spinner,
  StatusDot,
  Switch,
} from "@compozy/ui";

import { RemoveExtensionDialog, type InstalledExtensionView } from "@/systems/extensions";

import {
  installedDetailSearch,
  installedDisplayName,
  installedEntryId,
} from "../lib/marketplace-installed-view";
import { formatMarketplaceVersion } from "./marketplace-ui";

interface MarketplaceInstalledTrailProps {
  item: InstalledExtensionView;
  pending?: boolean;
  /** Search kept on the detail link so Back returns here with the query intact. */
  query?: string;
  /** Rows that provide an MCP server read their live definition only while this is on. */
  liveDataEnabled?: boolean;
  onUpdate: (item: InstalledExtensionView) => void;
  onToggleEnabled: (item: InstalledExtensionView, enabled: boolean) => void;
}

interface MarketplaceInstalledTrailControlsProps extends MarketplaceInstalledTrailProps {
  /** Replaces the default readiness word (status word · Authorize · runtime name). */
  leading?: ReactNode;
  /** Extra overflow items between View details and Remove. */
  menuItems?: ReactNode;
  onMenuOpenChange?: (open: boolean) => void;
}

/** The shared row controls: readiness · update · switch · overflow · typed-name remove. */
function MarketplaceInstalledTrailControls({
  item,
  pending = false,
  query,
  leading,
  menuItems,
  onMenuOpenChange,
  onUpdate,
  onToggleEnabled,
}: MarketplaceInstalledTrailControlsProps) {
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
      {leading !== undefined ? (
        leading
      ) : extension.missing_inputs.length > 0 ? (
        <span
          className="inline-flex items-center gap-1.5 text-eyebrow font-medium whitespace-nowrap text-warning"
          data-testid={`marketplace-installed-needs-configuration-${extension.name}`}
        >
          <StatusDot aria-hidden="true" size="sm" tone="warning" />
          Needs configuration
        </span>
      ) : null}
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
      <DropdownMenu onOpenChange={onMenuOpenChange}>
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
          {menuItems}
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

export { MarketplaceInstalledTrailControls };
export type { MarketplaceInstalledTrailControlsProps, MarketplaceInstalledTrailProps };
