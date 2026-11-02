import { queryOptions } from "@tanstack/react-query";

import { fetchPermissionEntries, fetchPermissionRollup } from "../adapters/activity-api";
import { getHomeActivity, getHomeOverview } from "../adapters/overview-api";
import type { ProfileScopeParams } from "@/systems/profiles";

import type { HomeActivityFilter, HomeOverviewFilter } from "../types";
import type { ActivityFilter } from "./activity-view";
import { dashboardKeys } from "./query-keys";

export function permissionRollupOptions(filter: ActivityFilter, profile: ProfileScopeParams) {
  return queryOptions({
    queryKey: dashboardKeys.permissionRollup(filter, profile),
    queryFn: ({ signal }) => fetchPermissionRollup(filter, profile, signal),
    staleTime: OVERVIEW_STALE_TIME,
    placeholderData: previous => previous,
  });
}

export function permissionEntriesOptions(filter: ActivityFilter, profile: ProfileScopeParams) {
  return queryOptions({
    queryKey: dashboardKeys.permissionEntries(filter, profile),
    queryFn: ({ signal }) => fetchPermissionEntries(filter, profile, signal),
    staleTime: OVERVIEW_STALE_TIME,
    placeholderData: previous => previous,
  });
}

export const OVERVIEW_STALE_TIME = 15_000;
export const OVERVIEW_REFETCH_INTERVAL = 30_000;
export const ACTIVITY_STALE_TIME = 5_000;
export const HOME_ACTIVITY_LIMIT = 30;

/**
 * Placeholder rule for the overview: while another usage window loads, the last window's overview
 * stays on screen instead of blanking the whole page into its skeleton. Only the usage window may
 * differ: a new project, profile or lens shows nothing stale.
 */
export function keepOverviewAcrossUsageWindow<TData>(
  previousData: TData | undefined,
  previousKey: readonly unknown[] | undefined,
  nextKey: readonly unknown[]
): TData | undefined {
  if (!previousKey) return undefined;
  return JSON.stringify(dashboardKeys.overviewPopulation(previousKey)) ===
    JSON.stringify(dashboardKeys.overviewPopulation(nextKey))
    ? previousData
    : undefined;
}

export function homeOverviewOptions(filters: HomeOverviewFilter = {}, enabled = true) {
  const queryKey = dashboardKeys.overview(filters);
  return queryOptions({
    queryKey,
    queryFn: ({ signal }) => getHomeOverview(filters, signal),
    placeholderData: (previousData, previousQuery) =>
      keepOverviewAcrossUsageWindow(previousData, previousQuery?.queryKey, queryKey),
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
