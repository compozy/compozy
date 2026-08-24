import { marketplaceEntryOptions } from "../lib/query-options";
import type { MarketplaceKind, MarketplaceListing } from "../types";
import type {
  MarketplaceActionControllerPhase,
  MarketplaceDialogSelection,
} from "./marketplace-action-controller-logic";

export function trustEntry(phase: MarketplaceActionControllerPhase): MarketplaceListing | null {
  return phase.status === "extensionTrust" || phase.status === "extensionTrustSubmitting"
    ? phase.entry
    : null;
}

export function marketplaceDialogSelection(
  phase: MarketplaceActionControllerPhase
): MarketplaceDialogSelection | null {
  return phase.status === "mcpInstall" ? phase.selection : null;
}

export function marketplaceDialogEntryOptions(
  selection: MarketplaceDialogSelection | null,
  workspaceId?: string | null
) {
  return marketplaceEntryOptions({
    entryId: selection?.entryId ?? "",
    installedName: selection?.installedName,
    kind: "mcp" as MarketplaceKind,
    workspaceId,
  });
}

export function trustError(phase: MarketplaceActionControllerPhase): string | null {
  return phase.status === "extensionTrust" ? phase.error : null;
}
