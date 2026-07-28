import { queryOptions } from "@tanstack/react-query";

import { getHomeActivity, getHomeOverview } from "../adapters/overview-api";
import type { HomeActivityFilter, HomeOverviewFilter } from "../types";
import { dashboardKeys } from "./query-keys";

export const OVERVIEW_STALE_TIME = 15_000;
export const OVERVIEW_REFETCH_INTERVAL = 30_000;
export const ACTIVITY_STALE_TIME = 5_000;
export const HOME_ACTIVITY_LIMIT = 30;

export function homeOverviewOptions(filters: HomeOverviewFilter = {}, enabled = true) {
  return queryOptions({
    queryKey: dashboardKeys.overview(filters),
    queryFn: ({ signal }) => getHomeOverview(filters, signal),
    staleTime: OVERVIEW_STALE_TIME,
    refetchInterval: OVERVIEW_REFETCH_INTERVAL,
    enabled,
  });
}

export function homeActivityOptions(filters: HomeActivityFilter = {}, enabled = true) {
  const withLimit: HomeActivityFilter = { limit: HOME_ACTIVITY_LIMIT, ...filters };
  return queryOptions({
    queryKey: dashboardKeys.activity(withLimit),
    queryFn: ({ signal }) => getHomeActivity(withLimit, signal),
    staleTime: ACTIVITY_STALE_TIME,
    enabled,
  });
}
