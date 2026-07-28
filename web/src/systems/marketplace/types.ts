import type { OperationQuery, OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export const MARKETPLACE_KINDS = ["mcp", "extension", "skill", "bundle"] as const;
export const MARKETPLACE_ROUTE_KINDS = ["skills", "mcps", "extensions", "bundles"] as const;

export type MarketplaceKind = (typeof MARKETPLACE_KINDS)[number];
export type MarketplaceRouteKind = (typeof MARKETPLACE_ROUTE_KINDS)[number];

const ROUTE_TO_API_KIND: Record<MarketplaceRouteKind, MarketplaceKind> = {
  bundles: "bundle",
  extensions: "extension",
  mcps: "mcp",
  skills: "skill",
};

const API_TO_ROUTE_KIND: Record<MarketplaceKind, MarketplaceRouteKind> = {
  bundle: "bundles",
  extension: "extensions",
  mcp: "mcps",
  skill: "skills",
};

export function marketplaceApiKindFor(kind: MarketplaceRouteKind): MarketplaceKind {
  return ROUTE_TO_API_KIND[kind];
}

export function marketplaceRouteKindFor(kind: MarketplaceKind): MarketplaceRouteKind {
  return API_TO_ROUTE_KIND[kind];
}

export function isMarketplaceRouteKind(value: unknown): value is MarketplaceRouteKind {
  return (
    typeof value === "string" && MARKETPLACE_ROUTE_KINDS.includes(value as MarketplaceRouteKind)
  );
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
export type BundlePreviewRequest = OperationRequestBody<"previewBundleActivation">;
export type BundlePreviewResponse = OperationResponse<"previewBundleActivation", 200>;
export type BundleActivationRequest = OperationRequestBody<"activateBundle">;
export type BundleActivationResponse = OperationResponse<"activateBundle", 201>;

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

interface MarketplaceEntryIdentityOptions extends MarketplaceScopeOptions {
  entryId: string;
}

export type MarketplaceEntryOptions =
  | (MarketplaceEntryIdentityOptions & {
      kind: "bundle";
      installedName?: never;
    })
  | (MarketplaceEntryIdentityOptions & {
      kind: Exclude<MarketplaceKind, "bundle">;
      installedName?: string | null;
    });
