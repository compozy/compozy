import type { OperationQuery, OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type MarketplaceRefreshResponse = OperationResponse<"refreshMarketplaceCatalog", 200>;
export type ExtensionInstallRequest = OperationRequestBody<"installExtension">;
export type ExtensionInstallResponse = OperationResponse<"installExtension", 201>;
export type ExtensionUpdateRequest = OperationRequestBody<"updateExtension">;

export interface MarketplaceScopeOptions {
  workspaceId?: string | null;
}
export type MarketplaceCatalogResponse = OperationResponse<"listMarketplace", 200>;
export type MarketplaceCatalogEntryResponse = OperationResponse<"getMarketplaceCatalogEntry", 200>;
export type MarketplaceExtensionServer = NonNullable<
  MarketplaceCatalogEntryResponse["extension"]
>["mcp_servers"][number];
export type MarketplaceCatalogQuery = OperationQuery<"listMarketplace">;
export type MarketplaceCatalogEntryQuery = OperationQuery<"getMarketplaceCatalogEntry">;

export interface MarketplaceCatalogOptions extends MarketplaceScopeOptions {
  q?: string | null;
  limit?: number;
  profileName?: string | null;
}
export interface MarketplaceCatalogPageOptions extends MarketplaceCatalogOptions {
  cursor?: string;
}
export interface MarketplaceCatalogEntryOptions extends MarketplaceScopeOptions {
  entryId: string;
  source?: string | null;
  installedName?: string | null;
  profileName?: string | null;
}

export type MarketplaceCatalogListing = MarketplaceCatalogResponse["items"][number];
export type ExtensionBatchUpdateRequest = OperationRequestBody<"updateExtensions">;
export type ExtensionBatchUpdateResponse = OperationResponse<"updateExtensions", 200>;

export type MarketplaceSourcesResponse = OperationResponse<"listMarketplaceSources", 200>;
export type MarketplaceSource = MarketplaceSourcesResponse["sources"][number];
export type MarketplaceSourceResponse = OperationResponse<"addMarketplaceSource", 201>;
export type MarketplaceSourcePreview = OperationResponse<"addMarketplaceSource", 200>;
export type AddMarketplaceSourceRequest = OperationRequestBody<"addMarketplaceSource">;
export type UpdateMarketplaceSourceRequest = OperationRequestBody<"updateMarketplaceSource">;
