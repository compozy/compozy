import { useQuery } from "@tanstack/react-query";

import { fetchDashboard } from "../adapters/dashboard-api";
import { dashboardKeys } from "../lib/query-keys";
import type { DashboardPayload } from "../types";

export function useDashboard(workspaceId: string | null) {
  return useQuery<DashboardPayload>({
    queryKey: dashboardKeys.byWorkspace(workspaceId ?? "none"),
    queryFn: () => {
      if (!workspaceId) {
        throw new Error("active workspace is required to load dashboard");
      }
      return fetchDashboard(workspaceId);
    },
    enabled: Boolean(workspaceId),
    refetchInterval: 3_000,
    refetchIntervalInBackground: false,
  });
}
