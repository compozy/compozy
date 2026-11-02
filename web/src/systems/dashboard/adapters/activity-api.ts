import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";
import type { ProfileScopeParams } from "@/systems/profiles";

import type { ActivityFilter } from "../lib/activity-view";
import { ACTIVITY_WINDOW_DAYS } from "../lib/activity-view";
import { DashboardApiError } from "./dashboard-api-errors";

function auditQuery(filter: ActivityFilter, profile: ProfileScopeParams) {
  return {
    days: String(ACTIVITY_WINDOW_DAYS),
    ...(filter === "unattended" ? { unattended: true } : {}),
    ...profile,
  };
}

export async function fetchPermissionRollup(
  filter: ActivityFilter,
  profile: ProfileScopeParams,
  signal?: AbortSignal
) {
  const { data, error, response } = await apiClient.GET("/api/permissions/rollup", {
    params: { query: auditQuery(filter, profile) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new DashboardApiError(
      defaultApiErrorMessage("Failed to fetch activity totals", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch activity totals");
}

export async function fetchPermissionEntries(
  filter: ActivityFilter,
  profile: ProfileScopeParams,
  signal?: AbortSignal
) {
  const { data, error, response } = await apiClient.GET("/api/permissions", {
    params: { query: auditQuery(filter, profile) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new DashboardApiError(
      defaultApiErrorMessage("Failed to fetch activity", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch activity").entries;
}
