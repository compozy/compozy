import { apiClient, apiRequestFailed } from "@/lib/api-client";

import type { NavCountFetchers } from "./nav-counts-store";

/**
 * Use a live-only stream for the process-wide navigation counters. Their
 * initial snapshots come from the owning APIs, so replaying the observability
 * history would add latency without improving correctness.
 */
export function observeStreamUrlForWorkspace(workspaceId: string): string {
  const query = new URLSearchParams({ replay: "false", workspace_id: workspaceId });
  return `/api/logs/stream?${query.toString()}`;
}

/** Fetchers for the initial and periodic navigation count snapshots. */
export function createDefaultFetchers(workspaceId?: string | null): NavCountFetchers {
  return {
    tasks: async signal => {
      const { data, error, response } = await apiClient.GET("/api/observe/tasks/dashboard", {
        params: workspaceId
          ? { query: { scope: "workspace" as const, workspace: workspaceId } }
          : undefined,
        signal,
      });
      if (apiRequestFailed(response, error)) {
        throw new Error(`nav-counts tasks snapshot failed: ${response.status}`);
      }
      const total = data?.dashboard?.totals?.tasks_total ?? 0;
      return { count: total };
    },
    jobs: async signal => {
      const { data, error, response } = await apiClient.GET("/api/automation/jobs", {
        params: workspaceId
          ? { query: { scope: "workspace" as const, workspace_id: workspaceId } }
          : undefined,
        signal,
      });
      if (apiRequestFailed(response, error)) {
        throw new Error(`nav-counts jobs snapshot failed: ${response.status}`);
      }
      return { count: data?.jobs?.length ?? 0 };
    },
    network: async signal => {
      const { data, error, response } = await apiClient.GET("/api/network/status", { signal });
      if (apiRequestFailed(response, error)) {
        throw new Error(`nav-counts network snapshot failed: ${response.status}`);
      }
      const network = data?.network;
      return { count: (network?.local_peers ?? 0) + (network?.channels ?? 0) };
    },
    triggers: async signal => {
      const { data, error, response } = await apiClient.GET("/api/automation/triggers", {
        params: workspaceId
          ? { query: { scope: "workspace" as const, workspace_id: workspaceId } }
          : undefined,
        signal,
      });
      if (apiRequestFailed(response, error)) {
        throw new Error(`nav-counts triggers snapshot failed: ${response.status}`);
      }
      return { count: data?.triggers?.length ?? 0 };
    },
    agents: async signal => {
      const { data, error, response } = await apiClient.GET("/api/agents", {
        params: workspaceId ? { query: { workspace: workspaceId } } : undefined,
        signal,
      });
      if (apiRequestFailed(response, error)) {
        throw new Error(`nav-counts agents snapshot failed: ${response.status}`);
      }
      return { count: data?.agents?.length ?? 0 };
    },
    knowledge: async signal => {
      const { data, error, response } = await apiClient.GET("/api/memory", {
        params: workspaceId
          ? { query: { scope: "workspace" as const, workspace_id: workspaceId } }
          : undefined,
        signal,
      });
      if (apiRequestFailed(response, error)) {
        throw new Error(`nav-counts knowledge snapshot failed: ${response.status}`);
      }
      return { count: data?.memories?.length ?? 0 };
    },
    skills: async signal => {
      const { data, error, response } = await apiClient.GET("/api/skills", {
        params: workspaceId ? { query: { workspace: workspaceId } } : undefined,
        signal,
      });
      if (apiRequestFailed(response, error)) {
        throw new Error(`nav-counts skills snapshot failed: ${response.status}`);
      }
      return { count: data?.skills?.length ?? 0 };
    },
    bridges: async signal => {
      const { data, error, response } = await apiClient.GET("/api/bridges", {
        params: workspaceId
          ? { query: { scope: "workspace" as const, workspace_id: workspaceId } }
          : undefined,
        signal,
      });
      if (apiRequestFailed(response, error)) {
        throw new Error(`nav-counts bridges snapshot failed: ${response.status}`);
      }
      return { count: data?.bridges?.length ?? 0 };
    },
  };
}
