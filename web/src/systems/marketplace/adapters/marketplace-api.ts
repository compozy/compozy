import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import { marketplaceApiError, MarketplaceApiError } from "./marketplace-api-error";
import type {
  MarketplaceEntryOptions,
  MarketplaceEntryQuery,
  MarketplaceEntryResponse,
  MarketplaceKindPageOptions,
  MarketplaceKindQuery,
  MarketplaceKindResponse,
  MarketplaceSearchOptions,
  MarketplaceSearchQuery,
  MarketplaceSearchResponse,
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

function searchQuery(options: MarketplaceSearchOptions): MarketplaceSearchQuery {
  return {
    q: normalizeOptionalText(options.q),
    limit: options.limit,
    ...scopeQuery(options.workspaceId),
  };
}

function kindQuery(options: MarketplaceKindPageOptions): MarketplaceKindQuery {
  return { ...searchQuery(options), cursor: normalizeOptionalText(options.cursor) };
}

function entryQuery(options: MarketplaceEntryOptions): MarketplaceEntryQuery {
  return {
    ...scopeQuery(options.workspaceId),
    ...(options.kind === "bundle"
      ? {}
      : { installed_name: normalizeOptionalText(options.installedName) }),
  };
}

export async function searchMarketplace(
  options: MarketplaceSearchOptions = {},
  signal?: AbortSignal
): Promise<MarketplaceSearchResponse> {
  const { data, error, response } = await apiClient.GET("/api/marketplace/search", {
    params: { query: searchQuery(options) },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw marketplaceApiError("Failed to search the marketplace", response, error);
  }

  return requireResponseData(data, response, "Failed to search the marketplace");
}

export async function browseMarketplaceKind(
  options: MarketplaceKindPageOptions,
  signal?: AbortSignal
): Promise<MarketplaceKindResponse> {
  const { data, error, response } = await apiClient.GET("/api/marketplace/{kind}", {
    params: {
      path: { kind: options.kind },
      query: kindQuery(options),
    },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw marketplaceApiError(`Failed to browse ${options.kind} entries`, response, error);
  }

  return requireResponseData(data, response, `Failed to browse ${options.kind} entries`);
}

export async function getMarketplaceEntry(
  options: MarketplaceEntryOptions,
  signal?: AbortSignal
): Promise<MarketplaceEntryResponse> {
  const entryId = options.entryId.trim();
  const { data, error, response } = await apiClient.GET("/api/marketplace/{kind}/{entry_id}", {
    params: {
      path: { kind: options.kind, entry_id: entryId },
      query: entryQuery(options),
    },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw marketplaceApiError(`Failed to load marketplace entry "${entryId}"`, response, error);
  }

  return requireResponseData(data, response, `Failed to load marketplace entry "${entryId}"`);
}
