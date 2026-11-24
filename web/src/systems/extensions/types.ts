import type { OperationRequestBody, OperationResponse } from "@/lib/api-contract";
import type { MarketplaceCatalogListing } from "@/systems/marketplace";

type ExtensionPayload = OperationResponse<"listExtensions", 200>["extensions"][number];
type ExtensionArrayField = "mcp_servers" | "inputs" | "missing_inputs";
export type ExtensionEntry = Omit<ExtensionPayload, ExtensionArrayField> & {
  [Field in ExtensionArrayField]: NonNullable<ExtensionPayload[Field]>;
};
/** Experimental: a dependency the instance no longer resolves (installed, enabled, visible, at `min_version`). */
export type ExtensionUnmetDependency = NonNullable<ExtensionPayload["unmet_dependencies"]>[number];
export type ExtensionProvenance = OperationResponse<"getExtensionProvenance", 200>["provenance"];
export type ExtensionUpdateRequest = OperationRequestBody<"updateExtension">;
export type ExtensionLogsSnapshot = OperationResponse<"getExtensionLogs", 200>;
export type ExtensionLogEntry = ExtensionLogsSnapshot["logs"][number];
export type ExtensionEnablement = OperationResponse<"setExtensionEnablement", 200>;
export type ExtensionInstallRequest = OperationRequestBody<"previewExtensionInstall">;
export type ExtensionInstallPreview = OperationResponse<"previewExtensionInstall", 200>;
/** One node of the experimental dependency graph an install preview resolved. */
export type ExtensionDependency = NonNullable<ExtensionInstallPreview["dependencies"]>[number];
export type ExtensionKitInventory = OperationResponse<"getExtensionInventory", 200>;
export type ExtensionKitItem = ExtensionKitInventory["items"][number];
/** One permanently removed built-in: a tombstone the daemon lists in its API order. */
export type ExtensionRemovedBuiltin = OperationResponse<
  "listRemovedBuiltinExtensions",
  200
>["removed"][number];
export type ExtensionRestoreResult = OperationResponse<"restoreExtension", 200>;
/** Setup view of one instance: declared inputs keyed by `state`, never values. */
export type ExtensionInputsState = OperationResponse<"getExtensionInputs", 200>;
export type ExtensionInputStatus = ExtensionInputsState["inputs"][number];
export type ExtensionSetInputsRequest = OperationRequestBody<"setExtensionInputs">;

/** Selects one daemon extension instance by optional workspace and profile axes. */
export interface ExtensionInstanceScope {
  workspaceId?: string | null;
  profileName?: string | null;
}

export interface InstalledExtensionView {
  extension: ExtensionEntry;
  listing: MarketplaceCatalogListing | null;
  updateAvailable: boolean;
}
