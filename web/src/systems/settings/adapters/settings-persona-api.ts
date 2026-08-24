import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  SettingsMutationResult,
  SettingsPersonaFilter,
  SettingsPersonaSection,
  SettingsUpdatePersonaRequest,
} from "../types";
import { SettingsApiError } from "./settings-api-error";
import { normalizeSettingsLayerFilter } from "./settings-layer-filter";

export async function getSettingsPersona(
  filter: SettingsPersonaFilter,
  signal?: AbortSignal
): Promise<SettingsPersonaSection> {
  const { data, error, response } = await apiClient.GET("/api/settings/persona", {
    params: { query: normalizeSettingsLayerFilter(filter) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load profile defaults", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load profile defaults");
}

export async function updateSettingsPersona(
  body: SettingsUpdatePersonaRequest,
  filter: SettingsPersonaFilter,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.PATCH("/api/settings/persona", {
    body,
    params: { query: normalizeSettingsLayerFilter(filter) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to update profile defaults", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update profile defaults");
}
