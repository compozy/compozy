import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  RolesStatusResponse,
  SettingsMutationResult,
  SettingsRolesSection,
  SettingsUpdateRolesRequest,
} from "../types";
import { SettingsApiError } from "./settings-api-error";

/**
 * Read-only effective role projection (`GET /api/roles`), resolved at global
 * scope. Drives resolution mode, badges, provenance, diagnostics, and the
 * null/"resolves at invocation" affordances — never the editable draft.
 */
export async function getRolesStatus(signal?: AbortSignal): Promise<RolesStatusResponse> {
  const { data, error, response } = await apiClient.GET("/api/roles", { signal });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load roles", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load roles");
}

/** Editable `[roles]` settings section (`GET /api/settings/roles`) — the draft source. */
export async function getSettingsRoles(signal?: AbortSignal): Promise<SettingsRolesSection> {
  const { data, error, response } = await apiClient.GET("/api/settings/roles", { signal });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load roles settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load roles settings");
}

/** Apply the full `[roles]` section through the typed Settings config plane. */
export async function updateSettingsRoles(
  body: SettingsUpdateRolesRequest,
  signal?: AbortSignal
): Promise<SettingsMutationResult> {
  const { data, error, response } = await apiClient.PATCH("/api/settings/roles", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to update roles settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update roles settings");
}
