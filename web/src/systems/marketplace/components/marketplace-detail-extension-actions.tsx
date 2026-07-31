import { ArrowUpCircle, MoreHorizontal } from "lucide-react";

import {
  Button,
  ConfirmDialog,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Eyebrow,
  Spinner,
  Switch,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import { ExtensionTrustBadges, type ExtensionEntry } from "@/systems/extensions";
import type { ExtensionTrustFacts } from "@/systems/extensions";

import { formatMarketplaceVersion } from "./marketplace-ui";

interface MarketplaceDetailExtensionActionsProps {
  catalogUpdateAvailable?: boolean;
  catalogVersion?: string;
  extension: ExtensionEntry;
  facts: ExtensionTrustFacts;
  isUpdatePending: boolean;
  onRequestProvenance: () => void;
  onRequestRemoval: () => void;
  onToggleEnabled: (enabled: boolean) => void;
  onUpdate: () => void;
  togglePending: boolean;
}

/**
 * Enable/disable and update act on the published registry row. A dev overlay shadows that row
 * without owning it, so neither control is offered while the workspace overlay is selected.
 */
export function MarketplaceDetailExtensionActions({
  catalogUpdateAvailable = false,
  catalogVersion,
  extension,
  facts,
  isUpdatePending,
  onRequestProvenance,
  onRequestRemoval,
  onToggleEnabled,
  onUpdate,
  togglePending,
}: MarketplaceDetailExtensionActionsProps) {
  const updateTarget = catalogVersion?.trim() || (extension.remote_version?.trim() ?? "");
  const displayUpdateTarget = formatMarketplaceVersion(updateTarget);
  const canUpdate = !facts.dev && (extension.update_available === true || catalogUpdateAvailable);
  return (
    <div className="flex flex-col gap-3 rounded-lg bg-canvas-soft px-4 py-3">
      <div className="flex items-center justify-between gap-3">
        <span className="flex items-center gap-2" data-testid="extension-enabled-toggle">
          <Eyebrow className="text-muted" id="marketplace-extension-enabled-label">
            {extension.enabled ? "Enabled" : "Disabled"}
          </Eyebrow>
          <EnabledSwitch
            checked={extension.enabled}
            dev={facts.dev}
            disabled={togglePending}
            onCheckedChange={onToggleEnabled}
          />
        </span>
        <span className="flex items-center gap-1.5">
          {canUpdate ? (
            <Button
              data-testid="extension-update-action"
              disabled={isUpdatePending}
              onClick={onUpdate}
              size="sm"
              type="button"
              variant="default"
            >
              {isUpdatePending ? (
                <Spinner aria-hidden="true" className="size-3" />
              ) : (
                <ArrowUpCircle aria-hidden="true" className="size-3.5" />
              )}
              {isUpdatePending
                ? "Updating…"
                : displayUpdateTarget
                  ? `Update to ${displayUpdateTarget}`
                  : "Update"}
            </Button>
          ) : null}
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  aria-label={`Actions for ${extension.name}`}
                  size="icon-sm"
                  variant="ghost"
                />
              }
            >
              <MoreHorizontal className="size-4" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {!facts.dev ? (
                <>
                  <DropdownMenuItem onClick={onRequestProvenance}>Provenance</DropdownMenuItem>
                  <DropdownMenuSeparator />
                </>
              ) : null}
              <DropdownMenuItem className="text-danger" onClick={onRequestRemoval}>
                {facts.dev ? "Unlink dev overlay…" : "Remove…"}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </span>
      </div>
      <div className="flex flex-wrap items-center gap-1.5" data-testid="extension-trust-badges">
        <ExtensionTrustBadges facts={facts} showRegistryTier={false} showSource />
      </div>
      {facts.dev ? (
        <p className="text-xs text-muted" data-testid="extension-dev-overlay-note">
          This workspace runs a linked dev generation. Enable, disable, and update stay on the
          published extension; switch to the global instance to manage them.
        </p>
      ) : null}
    </div>
  );
}

function EnabledSwitch({
  checked,
  dev,
  disabled,
  onCheckedChange,
}: {
  checked: boolean;
  dev: boolean;
  disabled: boolean;
  onCheckedChange: (enabled: boolean) => void;
}) {
  const control = (
    <Switch
      aria-labelledby="marketplace-extension-enabled-label"
      checked={checked}
      data-testid="extension-enabled-switch"
      disabled={disabled || dev}
      onCheckedChange={onCheckedChange}
    />
  );
  if (!dev) return control;
  return (
    <Tooltip>
      <TooltipTrigger render={<span className="inline-flex" />}>{control}</TooltipTrigger>
      <TooltipContent>
        Enabling and disabling apply to the published extension, not this workspace dev overlay.
      </TooltipContent>
    </Tooltip>
  );
}

export function ExtensionUpdateConsentDialog({
  error,
  extensionName,
  onConfirm,
  onOpenChange,
  open,
  pending,
  targetVersion,
}: {
  error?: string;
  extensionName: string;
  onConfirm: () => Promise<void>;
  onOpenChange: (open: boolean) => void;
  open: boolean;
  pending: boolean;
  targetVersion: string;
}) {
  return (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmButtonProps={{ "data-testid": "extension-update-consent-confirm" }}
      confirmLabel="Update anyway"
      contentProps={{ "data-testid": "extension-update-consent-dialog" }}
      description={`This extension is not registry-verified. Updating installs ${formatMarketplaceVersion(targetVersion) ?? "the newer release"} of ${extensionName} without a pinned checksum.`}
      error={error}
      isPending={pending}
      note="The daemon requires an explicit decision for every unverified install or update."
      noteTone="warning"
      onConfirm={onConfirm}
      onOpenChange={onOpenChange}
      open={open}
      title={`Update ${extensionName}?`}
      tone="warning"
    />
  );
}
