import { PROFILE_AGGREGATE } from "@/systems/profiles";
import type { HomeActivityFilter, HomeOverviewFilter } from "../types";

function normalizeText(value?: string | null): string {
  return typeof value === "string" ? value : "";
}

function normalizeNumber(value?: number): string {
  return value === undefined ? "" : String(value);
}

export const dashboardKeys = {
  all: ["dashboard"] as const,

  overviewRoot: () => [...dashboardKeys.all, "overview"] as const,
  overview: (filters: HomeOverviewFilter = {}) =>
    [
      ...dashboardKeys.overviewRoot(),
      normalizeText(filters.workspace),
      normalizeNumber(filters.usageWindow),
      // Two profiles reading the same window are two answers, never one entry.
      filters.allProfiles === true ? PROFILE_AGGREGATE : normalizeText(filters.profile),
    ] as const,

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
