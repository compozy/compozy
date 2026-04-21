import { daemonApiClient, requireData } from "@/lib/api-client";
import { ACTIVE_WORKSPACE_HEADER } from "@/systems/app-shell";

import type { DashboardPayload } from "../types";

export async function fetchDashboard(workspaceId: string): Promise<DashboardPayload> {
  const { data, error, response } = await daemonApiClient.GET("/api/ui/dashboard", {
    params: { header: { [ACTIVE_WORKSPACE_HEADER]: workspaceId } },
  });
  return requireData(data, response, "Failed to load dashboard", error).dashboard;
}
