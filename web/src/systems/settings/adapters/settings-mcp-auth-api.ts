import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  SettingsMCPAuthBeginRequest,
  SettingsMCPAuthBeginResponse,
  SettingsMCPAuthExchangeRequest,
  SettingsMCPAuthFilter,
  SettingsMCPAuthStatusResponse,
} from "../types";
import { SettingsApiError } from "./settings-api-error";
import { normalizeSettingsLayerFilter } from "./settings-layer-filter";

function normalizeMCPAuthFilter(filter: SettingsMCPAuthFilter) {
  return normalizeSettingsLayerFilter(filter);
}

export async function beginSettingsMCPAuth(
  name: string,
  filter: SettingsMCPAuthFilter,
  body: SettingsMCPAuthBeginRequest,
  signal?: AbortSignal
): Promise<SettingsMCPAuthBeginResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/settings/mcp-servers/{name}/auth/begin",
    { params: { path: { name }, query: normalizeMCPAuthFilter(filter) }, body, signal }
  );

  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage(`Failed to begin authorization for "${name}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to begin authorization for "${name}"`);
}

export async function exchangeSettingsMCPAuth(
  name: string,
  filter: SettingsMCPAuthFilter,
  body: SettingsMCPAuthExchangeRequest,
  signal?: AbortSignal
): Promise<SettingsMCPAuthStatusResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/settings/mcp-servers/{name}/auth/exchange",
    { params: { path: { name }, query: normalizeMCPAuthFilter(filter) }, body, signal }
  );

  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage(`Failed to complete authorization for "${name}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to complete authorization for "${name}"`);
}

export async function logoutSettingsMCPAuth(
  name: string,
  filter: SettingsMCPAuthFilter,
  signal?: AbortSignal
): Promise<SettingsMCPAuthStatusResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/settings/mcp-servers/{name}/auth/logout",
    { params: { path: { name }, query: normalizeMCPAuthFilter(filter) }, signal }
  );

  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage(`Failed to sign out "${name}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to sign out "${name}"`);
}
