import type { OperationRequestBody, OperationResponse } from "@/lib/api-contract";
import type { MarketplaceListing } from "@/systems/marketplace";

export type ExtensionEntry = OperationResponse<"listExtensions", 200>["extensions"][number];
export type ExtensionProvenance = OperationResponse<"getExtensionProvenance", 200>["provenance"];
export type ExtensionUpdateRequest = OperationRequestBody<"updateExtension">;
export type BundleActivation = OperationResponse<
  "listBundleActivations",
  200
>["activations"][number];
export type BundleActivationUpdateRequest = OperationRequestBody<"updateBundleActivation">;

export interface InstalledExtensionView {
  extension: ExtensionEntry;
  listing: MarketplaceListing | null;
  updateAvailable: boolean;
}
