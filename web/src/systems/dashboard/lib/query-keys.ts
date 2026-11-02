import { PROFILE_AGGREGATE, type ProfileScopeParams } from "@/systems/profiles";
import type { HomeActivityFilter, HomeOverviewFilter } from "../types";

function normalizeText(value?: string | null): string {
  return typeof value === "string" ? value : "";
}

function normalizeNumber(value?: number): string {
  return value === undefined ? "" : String(value);
}

/** Where the usage window sits in an overview key: after the root and the scope. */
const OVERVIEW_USAGE_WINDOW_SLOT = 3;

export const dashboardKeys = {
  all: ["dashboard"] as const,

  overviewRoot: () => [...dashboardKeys.all, "overview"] as const,
  overview: (filters: HomeOverviewFilter = {}) =>
    [
      ...dashboardKeys.overviewRoot(),
      // `@all` cannot be a workspace id, so the aggregate never shares an entry
      // with a project or with the omission read.
      filters.allWorkspaces === true ? "@all" : normalizeText(filters.workspace),
      normalizeNumber(filters.usageWindow),
      normalizeText(filters.receiptProfile ?? filters.profile),
      // Two profiles reading the same window are two answers, never one entry.
      filters.allProfiles === true ? PROFILE_AGGREGATE : normalizeText(filters.profile),
    ] as const,

  /**
   * An overview key without its usage window: the scope and profile lens a window switch leaves
   * unchanged. Two keys with the same population read the same project and profile.
   */
  overviewPopulation: (queryKey: readonly unknown[]) =>
    queryKey.filter((_, index) => index !== OVERVIEW_USAGE_WINDOW_SLOT),

  permissionRollup: (filter: string, profile: ProfileScopeParams) =>
    [...dashboardKeys.all, "permission-rollup", filter, profile] as const,
  permissionEntries: (filter: string, profile: ProfileScopeParams) =>
    [...dashboardKeys.all, "permission-entries", filter, profile] as const,

  activityRoot: () => [...dashboardKeys.all, "activity"] as const,
  activity: (filters: HomeActivityFilter = {}) =>
    [
      ...dashboardKeys.activityRoot(),
      normalizeText(filters.workspace_id),
      normalizeNumber(filters.limit),
      // The lens partitions the feed the same way it partitions the overview:
      // one profile's events and the labeled aggregate are two answers, so a
      // switch reads a different entry rather than inheriting the last one.
      filters.all_profiles === true ? PROFILE_AGGREGATE : normalizeText(filters.profile),
    ] as const,
};
