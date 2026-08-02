import type { OperationRequestBody, OperationResponse } from "@/lib/api-contract";
import type { MarketplaceListing } from "@/systems/marketplace";

export type ExtensionEntry = OperationResponse<"listExtensions", 200>["extensions"][number];
export type ExtensionProvenance = OperationResponse<"getExtensionProvenance", 200>["provenance"];
export type ExtensionUpdateRequest = OperationRequestBody<"updateExtension">;
export type ExtensionLogEntry = OperationResponse<"getExtensionLogs", 200>["logs"][number];
export type ExtensionEnableResult = OperationResponse<"enableExtension", 200>;
export type ExtensionInventory = OperationResponse<"getExtensionInventory", 200>;
export type ExtensionKitItem = ExtensionInventory["items"][number];

/** Selects one daemon extension instance; an absent workspace addresses the global published row. */
export interface ExtensionInstanceScope {
  workspaceId?: string | null;
}

export interface InstalledExtensionView {
  extension: ExtensionEntry;
  listing: MarketplaceListing | null;
  updateAvailable: boolean;
}
