import type { OperationRequestBody, OperationResponse } from "@/lib/api-contract";
import type { MarketplaceListing } from "@/systems/marketplace";

export type ExtensionEntry = OperationResponse<"listExtensions", 200>["extensions"][number];
export type ExtensionProvenance = OperationResponse<"getExtensionProvenance", 200>["provenance"];
export type ExtensionUpdateRequest = OperationRequestBody<"updateExtension">;
export type ExtensionLogsSnapshot = OperationResponse<"getExtensionLogs", 200>;
export type ExtensionLogEntry = ExtensionLogsSnapshot["logs"][number];
export type ExtensionEnablement = OperationResponse<"setExtensionEnablement", 200>;
export type ExtensionInstallRequest = OperationRequestBody<"previewExtensionInstall">;
export type ExtensionInstallPreview = OperationResponse<"previewExtensionInstall", 200>;
export type ExtensionKitInventory = OperationResponse<"getExtensionInventory", 200>;
export type ExtensionKitItem = ExtensionKitInventory["items"][number];

/** Selects one daemon extension instance by optional workspace and profile axes. */
export interface ExtensionInstanceScope {
  workspaceId?: string | null;
  profileName?: string | null;
}

export interface InstalledExtensionView {
  extension: ExtensionEntry;
  listing: MarketplaceListing | null;
  updateAvailable: boolean;
}
