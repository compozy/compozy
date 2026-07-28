import { normalizeAgentCatalogFilter } from "./agent-catalog-query";
import type { AgentCatalogStableFilter } from "../types";

export interface AgentHeartbeatStatusKeyOptions {
  workspaceId?: string | null;
  sessionId?: string | null;
  includeSessionHealth?: boolean;
  includeRecentWakeEvents?: boolean;
}

export const agentKeys = {
  all: ["agents"] as const,
  catalogs: () => [...agentKeys.all, "catalog"] as const,
  catalogByWorkspace: (workspace: string) => [...agentKeys.catalogs(), workspace.trim()] as const,
  catalog: (workspace: string, filters: AgentCatalogStableFilter = {}) =>
    [...agentKeys.catalogByWorkspace(workspace), normalizeAgentCatalogFilter(filters)] as const,
  lists: () => [...agentKeys.all, "list"] as const,
  list: (workspace?: string | null) => [...agentKeys.lists(), workspace ?? null] as const,
  detail: (name: string, workspace?: string | null) =>
    [...agentKeys.all, "detail", name, workspace ?? null] as const,
  soul: (name: string, workspace?: string | null) =>
    [...agentKeys.detail(name, workspace), "soul"] as const,
  soulHistory: (name: string, workspace?: string | null) =>
    [...agentKeys.soul(name, workspace), "history"] as const,
  heartbeat: (name: string, workspace?: string | null) =>
    [...agentKeys.detail(name, workspace), "heartbeat"] as const,
  heartbeatHistory: (name: string, workspace?: string | null) =>
    [...agentKeys.heartbeat(name, workspace), "history"] as const,
  heartbeatStatuses: (name: string, workspace?: string | null) =>
    [...agentKeys.heartbeat(name, workspace), "status"] as const,
  heartbeatStatus: (name: string, options: AgentHeartbeatStatusKeyOptions = {}) =>
    [
      ...agentKeys.heartbeatStatuses(name, options.workspaceId),
      options.sessionId ?? null,
      options.includeSessionHealth ?? null,
      options.includeRecentWakeEvents ?? null,
    ] as const,
};
