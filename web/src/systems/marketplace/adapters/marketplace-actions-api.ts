import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import { marketplaceApiError } from "./marketplace-api-error";
import type {
  BundleActivationRequest,
  BundleActivationResponse,
  BundlePreviewRequest,
  BundlePreviewResponse,
  ExtensionInstallRequest,
  ExtensionInstallResponse,
  MarketplaceKind,
  MarketplaceRefreshResponse,
  MCPInstallRequest,
  MCPInstallResponse,
  SkillInstallRequest,
  SkillInstallResponse,
  SkillUpdateRequest,
  SkillUpdateResponse,
} from "../types";

export async function refreshMarketplaceCatalog(
  kind?: Exclude<MarketplaceKind, "bundle">,
  signal?: AbortSignal
): Promise<MarketplaceRefreshResponse> {
  const { data, error, response } = await apiClient.POST("/api/marketplace/refresh", {
    params: { query: { kind } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw marketplaceApiError("Failed to refresh the marketplace", response, error);
  }

  return requireResponseData(data, response, "Failed to refresh the marketplace");
}

export async function installMarketplaceMCP(
  body: MCPInstallRequest,
  signal?: AbortSignal
): Promise<MCPInstallResponse> {
  const { data, error, response } = await apiClient.POST("/api/settings/mcp-servers/install", {
    body,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw marketplaceApiError("Failed to install the MCP server", response, error);
  }

  return requireResponseData(data, response, "Failed to install the MCP server");
}

export async function installMarketplaceSkill(
  body: SkillInstallRequest,
  signal?: AbortSignal
): Promise<SkillInstallResponse> {
  const { data, error, response } = await apiClient.POST("/api/skills/marketplace/install", {
    body,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw marketplaceApiError("Failed to install the skill", response, error);
  }

  return requireResponseData(data, response, "Failed to install the skill");
}

export async function updateMarketplaceSkill(
  body: SkillUpdateRequest,
  signal?: AbortSignal
): Promise<SkillUpdateResponse> {
  const { data, error, response } = await apiClient.POST("/api/skills/marketplace/update", {
    body,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw marketplaceApiError("Failed to update the skill", response, error);
  }

  return requireResponseData(data, response, "Failed to update the skill");
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

export async function previewMarketplaceBundle(
  body: BundlePreviewRequest,
  signal?: AbortSignal
): Promise<BundlePreviewResponse> {
  const { data, error, response } = await apiClient.POST("/api/bundles/preview", {
    body,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw marketplaceApiError("Failed to preview the bundle", response, error);
  }

  return requireResponseData(data, response, "Failed to preview the bundle");
}

export async function activateMarketplaceBundle(
  body: BundleActivationRequest,
  signal?: AbortSignal
): Promise<BundleActivationResponse> {
  const { data, error, response } = await apiClient.POST("/api/bundles/activations", {
    body,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw marketplaceApiError("Failed to activate the bundle", response, error);
  }

  return requireResponseData(data, response, "Failed to activate the bundle");
}
