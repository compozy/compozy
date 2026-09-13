import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import { marketplaceApiError, MarketplaceApiError } from "./marketplace-api-error";
import type {
  MarketplaceCatalogPageOptions,
  MarketplaceCatalogResponse,
  MarketplaceCatalogEntryOptions,
  MarketplaceCatalogEntryResponse,
  MarketplaceCatalogQuery,
  MarketplaceCatalogEntryQuery,
} from "../types";

export { MarketplaceApiError };

function normalizeOptionalText(value?: string | null): string | undefined {
  const normalized = value?.trim();
  return normalized === "" ? undefined : normalized;
}

function scopeQuery(workspaceId?: string | null) {
  const normalizedWorkspaceId = normalizeOptionalText(workspaceId);
  return normalizedWorkspaceId
    ? ({ scope: "workspace", workspace_id: normalizedWorkspaceId } as const)
    : ({ scope: "global" } as const);
}

export async function browseMarketplace(
  options: MarketplaceCatalogPageOptions = {},
  signal?: AbortSignal
): Promise<MarketplaceCatalogResponse> {
  const query: MarketplaceCatalogQuery = {
    q: normalizeOptionalText(options.q),
    limit: options.limit,
    cursor: normalizeOptionalText(options.cursor),
    profile: normalizeOptionalText(options.profileName),
    ...scopeQuery(options.workspaceId),
  };
  const { data, error, response } = await apiClient.GET("/api/marketplace", {
    params: { query },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw marketplaceApiError("Failed to load the marketplace", response, error);
  }
  return requireResponseData(data, response, "Failed to load the marketplace");
}

export async function getMarketplaceCatalogEntry(
  options: MarketplaceCatalogEntryOptions,
  signal?: AbortSignal
): Promise<MarketplaceCatalogEntryResponse> {
  const query: MarketplaceCatalogEntryQuery = {
    source: normalizeOptionalText(options.source),
    installed_name: normalizeOptionalText(options.installedName),
    profile: normalizeOptionalText(options.profileName),
    ...scopeQuery(options.workspaceId),
  };
  const { data, error, response } = await apiClient.GET("/api/marketplace/entries/{entry_id}", {
    params: { path: { entry_id: options.entryId.trim() }, query },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw marketplaceApiError("Failed to load the marketplace entry", response, error);
  }
  return requireResponseData(data, response, "Failed to load the marketplace entry");
}
