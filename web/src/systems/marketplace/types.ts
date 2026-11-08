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

export type MarketplaceSectionsResponse = OperationResponse<"getMarketplaceSections", 200>;
export type MarketplaceSection = MarketplaceSectionsResponse["sections"][number];
export type MarketplaceSectionsQuery = OperationQuery<"getMarketplaceSections">;
export type MarketplaceCategoryCount = MarketplaceCatalogResponse["categories"][number];
export type ExtensionEnableError = NonNullable<ExtensionInstallResponse["enable_error"]>;

/** The installed instance a Turn on / Retry addresses. */
export interface MarketplaceEnableTarget {
  name: string;
  profileName: string;
  workspaceId: string | null;
}
/** The installed instance an Add produced, kept with the cause its enable reported. */
export interface MarketplaceEnableFailure extends MarketplaceEnableTarget {
  cause: ExtensionEnableError;
}
/** Enable failures of one destination (workspace + profile), keyed by catalog origin. */
export type MarketplaceEnableFailures = Readonly<Record<string, MarketplaceEnableFailure>>;
/** The destination identity an install, enable or failure belongs to. */
export interface MarketplaceDestinationScope {
  profileName?: string | null;
  workspaceId?: string | null;
}

export interface MarketplaceCatalogOptions extends MarketplaceScopeOptions {
  q?: string | null;
  /** CompozyOS category id; the daemon scopes paging, search and counts to it. */
  category?: string | null;
  limit?: number;
  profileName?: string | null;
}
export interface MarketplaceSectionsOptions extends MarketplaceScopeOptions {
  q?: string | null;
  perSection?: number;
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
