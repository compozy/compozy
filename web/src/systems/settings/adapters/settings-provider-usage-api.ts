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

export async function updateSettingsProviderUsage(
  body: SettingsUpdateProviderUsageRequest,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient
    .PATCH("/api/settings/provider-usage", { body, signal })
    .catch(throwSettingsSaveTransportError);
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to update provider usage settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update provider usage settings");
}
