import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  SettingsMutationResult,
  SettingsProviderUsageSection,
  SettingsUpdateProviderUsageRequest,
} from "../types";
import { SettingsApiError, throwSettingsSaveTransportError } from "./settings-api-error";

export async function getSettingsProviderUsage(
  signal?: AbortSignal
): Promise<SettingsProviderUsageSection> {
  const { data, error, response } = await apiClient.GET("/api/settings/provider-usage", {
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load provider usage settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load provider usage settings");
}

/**
 * Saves the machine-wide section. `collectionProfile` names the profile whose
 * Claude collection a Keychain consent change starts; it scopes nothing else.
 */
export async function updateSettingsProviderUsage(
  body: SettingsUpdateProviderUsageRequest,
  collectionProfile: string,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient
    .PATCH("/api/settings/provider-usage", {
      body,
      params: { query: { collection_profile: collectionProfile } },
      signal,
    })
    .catch(throwSettingsSaveTransportError);
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to update provider usage settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update provider usage settings");
}
