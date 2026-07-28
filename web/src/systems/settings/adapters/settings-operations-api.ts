import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  ConfigApplyRecordsResponse,
  SettingsApplyRecordsFilter,
  SettingsApplyResponse,
  SettingsRestartResponse,
  SettingsRestartStatus,
} from "../types";
import { normalizeOptionalText, SettingsApiError } from "./settings-api-error";

function normalizeApplyRecordsFilter(filter: SettingsApplyRecordsFilter = {}) {
  return {
    status: filter.status,
    actor: normalizeOptionalText(filter.actor),
    limit: filter.limit,
  };
}

export async function triggerSettingsRestart(
  signal?: AbortSignal
): Promise<SettingsRestartResponse> {
  const { data, error, response } = await apiClient.POST("/api/settings/actions/restart", {
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to trigger daemon restart", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to trigger daemon restart");
}

export async function reloadSettings(signal?: AbortSignal): Promise<SettingsApplyResponse> {
  const { data, error, response } = await apiClient.POST("/api/settings/reload", { signal });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to reload settings", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to reload settings");
}

export async function listSettingsApplyRecords(
  filter: SettingsApplyRecordsFilter = {},
  signal?: AbortSignal
): Promise<ConfigApplyRecordsResponse> {
  const { data, error, response } = await apiClient.GET("/api/settings/apply", {
    params: { query: normalizeApplyRecordsFilter(filter) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SettingsApiError(
      defaultApiErrorMessage("Failed to load config apply records", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load config apply records");
}

export async function getSettingsRestartStatus(
  operationId: string,
  signal?: AbortSignal
): Promise<SettingsRestartStatus> {
  const { data, error, response } = await apiClient.GET(
    "/api/settings/actions/restart/{operation_id}",
    { params: { path: { operation_id: operationId } }, signal }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new SettingsApiError(`Restart operation not found: ${operationId}`, 404);
    }
    throw new SettingsApiError(
      defaultApiErrorMessage(`Failed to load restart status for "${operationId}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to load restart status for "${operationId}"`);
}

export const OBSERVABILITY_LOG_TAIL_PATH = "/api/settings/observability/log-tail" as const;

export function settingsObservabilityLogTailPath(): string {
  return OBSERVABILITY_LOG_TAIL_PATH;
}
