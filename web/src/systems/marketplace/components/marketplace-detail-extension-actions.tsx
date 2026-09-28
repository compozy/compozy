import { MoreHorizontal } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  PropertyRow,
  Switch,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import { type ExtensionEntry, type ExtensionTrustFacts } from "@/systems/extensions";

interface MarketplaceDetailExtensionActionsProps {
  extension: ExtensionEntry;
  facts: ExtensionTrustFacts;
  onRequestProvenance: () => void;
  onRequestRemoval: () => void;
  onToggleEnabled: (enabled: boolean) => void;
  togglePending: boolean;
}

/**
 * Rail Manage controls: the enable switch and the overflow actions. Update is
 * the OS-head primary action, never duplicated here. A dev overlay shadows the
 * published row without owning it, so the switch is withheld while the
 * workspace overlay is selected. Trust reads as one plain word; the badges live under Advanced.
 */
export function MarketplaceDetailExtensionActions({
  extension,
  facts,
  onRequestProvenance,
  onRequestRemoval,
  onToggleEnabled,
  togglePending,
}: MarketplaceDetailExtensionActionsProps) {
  const trustWord = extensionTrustWord(facts);
  return (
    <div className="flex flex-col gap-2 px-3.5 pb-1.5">
      <div className="flex items-center gap-1.5" data-testid="extension-enabled-toggle">
        <PropertyRow
          className="min-w-0 flex-1"
          editor={
            <EnabledSwitch
              checked={extension.enabled}
              dev={facts.dev}
              disabled={togglePending}
              onCheckedChange={onToggleEnabled}
            />
          }
          label={<span id="marketplace-extension-enabled-label">Enabled</span>}
        />
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button aria-label={`Actions for ${extension.name}`} size="icon-sm" variant="ghost" />
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
              {facts.dev ? "Unlink local copy…" : "Remove…"}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      {trustWord ? (
        <p className="text-form-label text-subtle" data-testid="extension-trust-word">
          {trustWord}
        </p>
      ) : null}
      {facts.dev ? (
        <p className="text-form-label text-muted" data-testid="extension-dev-overlay-note">
          This project runs a local development copy. Turning it on or off and updating apply to the
          published extension; open the version installed for all projects to manage them.
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
        Turning it on or off applies to the published extension, not this local development copy.
      </TooltipContent>
    </Tooltip>
  );
}

function extensionTrustWord(facts: ExtensionTrustFacts): string | null {
  if (facts.dev) return "Local development copy";
  if (facts.checksumVerified) return "Verified by CompozyOS";
  if (facts.digestMatched) return "Integrity checked";
  return null;
}
