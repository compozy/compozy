import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

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

export function buildHomeLogsStreamUrl(workspaceId: string): string {
  const params = new URLSearchParams();
  const workspace = workspaceId.trim();
  if (workspace !== "") {
    params.set("workspace_id", workspace);
  }
  const query = params.toString();
  return query === "" ? "/api/logs/stream" : `/api/logs/stream?${query}`;
}
