import type { InstalledExtensionView } from "@/systems/extensions";

import type { MarketplaceCatalogListing } from "../types";

/**
 * The single action a catalog row shows (US-007). `installed` is the honest fallback for a row the
 * daemon reports installed whose instance is not in this window's inventory (another scope): it
 * never claims Ready without the inventory confirming it.
 */
export type MarketplaceSetupState =
  | "add"
  | "adding"
  | "setup"
  | "ready"
  | "turn-on"
  | "enable-failed"
  | "installed"
  | "unknown";

export interface MarketplaceSetupInput {
  entry: Pick<MarketplaceCatalogListing, "installed" | "installed_name">;
  /** The inventory item joined to the row, `null` when the loaded inventory has none. */
  item: InstalledExtensionView | null;
  inventoryPending: boolean;
  pending?: boolean;
  /** The last Add of this row installed it but could not turn it on. */
  enableFailed?: boolean;
}

/** What is left before an installed extension can run: inputs first, then sign-in. */
export function installedSetupNeeds(item: InstalledExtensionView): {
  missingInputs: readonly string[];
  needsAuthorization: InstalledExtensionView["extension"]["mcp_servers"];
} {
  return {
    missingInputs: item.extension.missing_inputs,
    needsAuthorization: item.extension.mcp_servers.filter(
      server => server.status === "needs_authorization"
    ),
  };
}

/** Pure mapping from a listing and the installed inventory to the row's one action. */
export function marketplaceSetupState({
  entry,
  item,
  inventoryPending,
  pending = false,
  enableFailed = false,
}: MarketplaceSetupInput): MarketplaceSetupState {
  if (pending) return "adding";
  if (!entry.installed) return "add";
  if (inventoryPending) return "unknown";
  if (!item) return "installed";
  if (!item.extension.enabled) return enableFailed ? "enable-failed" : "turn-on";
  const needs = installedSetupNeeds(item);
  if (needs.missingInputs.length > 0 || needs.needsAuthorization.length > 0) return "setup";
  return "ready";
}

/** The inventory item a catalog row resolves to: its daemon-joined installed name. */
export function findInstalledItem(
  entry: Pick<MarketplaceCatalogListing, "installed" | "installed_name">,
  items: readonly InstalledExtensionView[]
): InstalledExtensionView | null {
  const name = entry.installed ? entry.installed_name?.trim() : "";
  if (!name) return null;
  return items.find(item => item.extension.name === name) ?? null;
}
