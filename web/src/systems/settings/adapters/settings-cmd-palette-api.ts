import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  SettingsCmdPaletteFilter,
  SettingsCmdPaletteSection,
  SettingsUpdateCmdPaletteFilter,
  SettingsUpdateCmdPaletteRequest,
} from "../types";
import { normalizeOptionalText, SettingsApiError } from "./settings-api-error";

function normalizeSettingsCmdPaletteFilter(
  filter: SettingsCmdPaletteFilter | SettingsUpdateCmdPaletteFilter = {}
) {
  return {
    scope: filter.scope,
    workspace_id: normalizeOptionalText(filter.workspace_id),
    profile: normalizeOptionalText(filter.profile),
  };
}

export async function getSettingsCmdPalette(
  filter: SettingsCmdPaletteFilter = {},
  signal?: AbortSignal
): Promise<SettingsCmdPaletteSection> {
  const { data, error, response } = await apiClient.GET("/api/settings/cmd-palette", {
    params: { query: normalizeSettingsCmdPaletteFilter(filter) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load command palette settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load command palette settings");
}

export async function updateSettingsCmdPalette(
  body: SettingsUpdateCmdPaletteRequest,
  filter: SettingsUpdateCmdPaletteFilter = {},
  signal?: AbortSignal
): Promise<SettingsCmdPaletteSection> {
  const { data, error, response } = await apiClient.PATCH("/api/settings/cmd-palette", {
    body,
    params: { query: normalizeSettingsCmdPaletteFilter(filter) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to update command palette settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update command palette settings");
}
