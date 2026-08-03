import type {
  OperationPath,
  OperationQuery,
  OperationRequestBody,
  OperationResponse,
} from "@/lib/api-contract";

export type MarketplaceKind = OperationPath<"browseMarketplaceKind">["kind"];

const MARKETPLACE_ROUTE_BY_KIND = {
  extension: "extensions",
  mcp: "mcps",
  skill: "skills",
} as const satisfies Record<MarketplaceKind, string>;

export type MarketplaceRouteKind = (typeof MARKETPLACE_ROUTE_BY_KIND)[MarketplaceKind];

export const MARKETPLACE_KINDS = Object.keys(
  MARKETPLACE_ROUTE_BY_KIND
) as readonly MarketplaceKind[];
export const MARKETPLACE_ROUTE_KINDS: readonly MarketplaceRouteKind[] =
  Object.values(MARKETPLACE_ROUTE_BY_KIND);

const ROUTE_TO_API_KIND = Object.fromEntries(
  MARKETPLACE_KINDS.map(kind => [MARKETPLACE_ROUTE_BY_KIND[kind], kind])
) as Record<MarketplaceRouteKind, MarketplaceKind>;

export function marketplaceApiKindFor(kind: MarketplaceRouteKind): MarketplaceKind {
  return ROUTE_TO_API_KIND[kind];
}

export function marketplaceRouteKindFor(kind: MarketplaceKind): MarketplaceRouteKind {
  return MARKETPLACE_ROUTE_BY_KIND[kind];
}

export function isMarketplaceRouteKind(value: unknown): value is MarketplaceRouteKind {
  return typeof value === "string" && Object.hasOwn(ROUTE_TO_API_KIND, value);
}

export type MarketplaceSearchResponse = OperationResponse<"searchMarketplace", 200>;
export type MarketplaceKindResponse = OperationResponse<"browseMarketplaceKind", 200>;
export type MarketplaceEntryResponse = OperationResponse<"getMarketplaceEntry", 200>;
export type MarketplaceRefreshResponse = OperationResponse<"refreshMarketplaceCatalog", 200>;
export type MarketplaceListing = MarketplaceKindResponse["items"][number];
export type MarketplaceKindResult = MarketplaceSearchResponse["kinds"][number];

export type MarketplaceSearchQuery = OperationQuery<"searchMarketplace">;
export type MarketplaceKindQuery = OperationQuery<"browseMarketplaceKind">;
export type MarketplaceEntryQuery = OperationQuery<"getMarketplaceEntry">;

export type MCPInstallRequest = OperationRequestBody<"installSettingsMCPServer">;
export type MCPInstallResponse = OperationResponse<"installSettingsMCPServer", 200>;
export type SkillInstallRequest = OperationRequestBody<"installSkillMarketplace">;
export type SkillInstallResponse = OperationResponse<"installSkillMarketplace", 200>;
export type SkillUpdateRequest = OperationRequestBody<"updateSkillMarketplace">;
export type SkillUpdateResponse = OperationResponse<"updateSkillMarketplace", 200>;
export type ExtensionInstallRequest = OperationRequestBody<"installExtension">;
export type ExtensionInstallResponse = OperationResponse<"installExtension", 201>;
export type ExtensionUpdateRequest = OperationRequestBody<"updateExtension">;

export interface MarketplaceScopeOptions {
  workspaceId?: string | null;
}

export interface MarketplaceSearchOptions extends MarketplaceScopeOptions {
  q?: string | null;
  limit?: number;
}

export interface MarketplaceKindOptions extends MarketplaceSearchOptions {
  kind: MarketplaceKind;
}

export interface MarketplaceKindPageOptions extends MarketplaceKindOptions {
  cursor?: string;
}

export interface MarketplaceEntryOptions extends MarketplaceScopeOptions {
  entryId: string;
  kind: MarketplaceKind;
  installedName?: string | null;
}
