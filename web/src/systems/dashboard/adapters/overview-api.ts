import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";
import type { ProfileScopeParams } from "@/systems/profiles";

import type {
  HomeActivityEvent,
  HomeActivityFilter,
  HomeOverview,
  HomeOverviewFilter,
  HomeOverviewWireFilter,
  HomeUsageWindow,
} from "../types";
import { DashboardApiError, normalizeOptionalText } from "./dashboard-api-errors";

const USAGE_WINDOW_WIRE = {
  7: "7",
  30: "30",
  90: "90",
} as const satisfies Record<HomeUsageWindow, NonNullable<HomeOverviewWireFilter["usage_window"]>>;

function normalizeOverviewFilter(filters: HomeOverviewFilter = {}): HomeOverviewWireFilter {
  return {
    workspace: normalizeOptionalText(filters.workspace),
    usage_window:
      filters.usageWindow === undefined ? undefined : USAGE_WINDOW_WIRE[filters.usageWindow],
    ...(filters.allProfiles
      ? { all_profiles: true }
      : { profile: normalizeOptionalText(filters.profile) }),
  };
}

export async function getHomeOverview(
  filters: HomeOverviewFilter = {},
  signal?: AbortSignal
): Promise<HomeOverview> {
  const { data, error, response } = await apiClient.GET("/api/observe/overview", {
    params: { query: normalizeOverviewFilter(filters) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new DashboardApiError(
      defaultApiErrorMessage("Failed to fetch home overview", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch home overview").overview;
}

export async function getHomeActivity(
  filters: HomeActivityFilter = {},
  signal?: AbortSignal
): Promise<HomeActivityEvent[]> {
  const { data, error, response } = await apiClient.GET("/api/logs", {
    params: {
      query: {
        workspace_id: normalizeOptionalText(filters.workspace_id),
        limit: filters.limit,
        // Activity is a profile-owned read: omitting the selector resolves to
        // `default`, so the aggregate view has to say so on the wire.
        ...(filters.all_profiles
          ? { all_profiles: true }
          : { profile: normalizeOptionalText(filters.profile) }),
      },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new DashboardApiError(
      defaultApiErrorMessage("Failed to fetch home activity", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch home activity").events;
}

/**
 * The stream carries the same read scope as the activity list it feeds, so the
 * two can never disagree about which owners are in view. The returned string is
 * also the stream's identity: a lens change produces a different URL, which is
 * what closes the previous source instead of leaving it feeding the new lens.
 */
export function buildHomeLogsStreamUrl(workspaceId: string, scope: ProfileScopeParams): string {
  const params = new URLSearchParams();
  const workspace = workspaceId.trim();
  if (workspace !== "") {
    params.set("workspace_id", workspace);
  }
  if ("all_profiles" in scope) params.set("all_profiles", "true");
  else params.set("profile", scope.profile);
  return `/api/logs/stream?${params.toString()}`;
}
