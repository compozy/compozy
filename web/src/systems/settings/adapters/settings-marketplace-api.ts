import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";
import type {
  SettingsMarketplaceSection,
  SettingsUpdateMarketplaceRequest,
  SettingsMutationResult,
} from "../types";
import { SettingsApiError } from "./settings-api-error";

export async function getSettingsMarketplace(
  signal?: AbortSignal
): Promise<SettingsMarketplaceSection> {
  const { data, error, response } = await apiClient.GET("/api/settings/marketplace", { signal });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load marketplace settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load marketplace settings");
}

export async function updateSettingsMarketplace(
  body: SettingsUpdateMarketplaceRequest,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.PATCH("/api/settings/marketplace", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to update marketplace settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update marketplace settings");
}
