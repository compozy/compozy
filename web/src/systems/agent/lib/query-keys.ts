import { normalizeAgentCatalogFilter } from "./agent-catalog-query";
import type { AgentCatalogStableFilter } from "../types";

export interface AgentHeartbeatStatusKeyOptions {
  profile?: string;
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
  list: (workspace?: string | null, profile = "default") =>
    [...agentKeys.lists(), workspace ?? null, profile] as const,
  detail: (name: string, workspace?: string | null, profile = "default") =>
    [...agentKeys.all, "detail", name, workspace ?? null, profile] as const,
  soul: (name: string, workspace?: string | null, profile = "default") =>
    [...agentKeys.detail(name, workspace, profile), "soul"] as const,
  soulHistory: (name: string, workspace?: string | null, profile = "default") =>
    [...agentKeys.soul(name, workspace, profile), "history"] as const,
  heartbeat: (name: string, workspace?: string | null, profile = "default") =>
    [...agentKeys.detail(name, workspace, profile), "heartbeat"] as const,
  heartbeatHistory: (name: string, workspace?: string | null, profile = "default") =>
    [...agentKeys.heartbeat(name, workspace, profile), "history"] as const,
  heartbeatStatuses: (name: string, workspace?: string | null, profile = "default") =>
    [...agentKeys.heartbeat(name, workspace, profile), "status"] as const,
  heartbeatStatus: (name: string, options: AgentHeartbeatStatusKeyOptions = {}) =>
    [
      ...agentKeys.heartbeatStatuses(name, options.workspaceId, options.profile),
      options.sessionId ?? null,
      options.includeSessionHealth ?? null,
      options.includeRecentWakeEvents ?? null,
    ] as const,
};
