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
    ] as const,

  activityRoot: () => [...dashboardKeys.all, "activity"] as const,
  activity: (filters: HomeActivityFilter = {}) =>
    [
      ...dashboardKeys.activityRoot(),
      normalizeText(filters.workspace_id),
      normalizeNumber(filters.limit),
    ] as const,
};
