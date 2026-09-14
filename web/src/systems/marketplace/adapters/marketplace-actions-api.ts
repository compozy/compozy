import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import { marketplaceApiError } from "./marketplace-api-error";
import type {
  ExtensionBatchUpdateRequest,
  ExtensionBatchUpdateResponse,
  ExtensionInstallRequest,
  ExtensionInstallResponse,
  MarketplaceRefreshResponse,
} from "../types";

export async function refreshMarketplaceCatalog(
  signal?: AbortSignal
): Promise<MarketplaceRefreshResponse> {
  const { data, error, response } = await apiClient.POST("/api/marketplace/refresh", {
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw marketplaceApiError("Failed to refresh the marketplace", response, error);
  }

  return requireResponseData(data, response, "Failed to refresh the marketplace");
}

export async function installMarketplaceExtension(
  body: ExtensionInstallRequest,
  signal?: AbortSignal
): Promise<ExtensionInstallResponse> {
  const { data, error, response } = await apiClient.POST("/api/extensions", { body, signal });

  if (apiRequestFailed(response, error)) {
    throw marketplaceApiError("Failed to install the extension", response, error);
  }

  return requireResponseData(data, response, "Failed to install the extension");
}

export async function updateMarketplaceExtensions(
  body: ExtensionBatchUpdateRequest,
  signal?: AbortSignal
): Promise<ExtensionBatchUpdateResponse> {
  const { data, error, response } = await apiClient.POST("/api/extensions/update", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw marketplaceApiError("Failed to update extensions", response, error);
  }
  return requireResponseData(data, response, "Failed to update extensions");
}
