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
  /** Every All projects catalog read, across Project-filter selections. */
  aggregates: () => [...agentKeys.catalogs(), "@aggregate"] as const,
  /** The All projects catalog, per Project-filter selection; lives under `catalogs()`. */
  aggregate: (workspaceIds: readonly string[], filters: AgentCatalogStableFilter = {}) =>
    [
      ...agentKeys.aggregates(),
      [...workspaceIds].sort().join(","),
      normalizeAgentCatalogFilter(filters),
    ] as const,
  instances: () => [...agentKeys.all, "instances"] as const,
  instanceLists: () => [...agentKeys.instances(), "list"] as const,
  /** The view's always-on agents across every scope; one read shared by every row. */
  instanceList: (view: string, profiles: readonly string[]) =>
    [...agentKeys.instanceLists(), view, profiles.join(",")] as const,
  instanceAway: (id: string) => [...agentKeys.instances(), id, "away"] as const,
  instanceCheckIns: (id: string) => [...agentKeys.all, "instance-check-ins", id] as const,
  instanceDayCheckIns: (id: string) => [...agentKeys.instanceCheckIns(id), "day"] as const,
  instanceOccurrence: (id: string, occurrence: string) =>
    [...agentKeys.all, "instance-occurrence", id, occurrence] as const,
  instanceReports: (id: string) => [...agentKeys.all, "instance-reports", id] as const,
  skillReviews: () => [...agentKeys.all, "skill-review"] as const,
  skillReview: (id: string, profile = "") => [...agentKeys.skillReviews(), id, profile] as const,
  instanceIndeterminateReports: (id: string) =>
    [...agentKeys.instanceReports(id), "indeterminate"] as const,
  /** Every kept report with its live picks and tool calls; transcript parts select theirs. */
  instanceProjectedReports: (id: string) =>
    [...agentKeys.instanceReports(id), "projected"] as const,
  instanceOccurrenceCheckIn: (id: string, occurrence: string) =>
    [...agentKeys.instanceCheckIns(id), "occurrence", occurrence] as const,
  /** Definitions a scope can start: one `group=agent` read per scope and profile. */
  alwaysOnCatalog: (workspace: string, profile = "default") =>
    [...agentKeys.catalogs(), "@always-on", workspace, profile] as const,
  /** One card per agent for a scope; `""` is every project and User. */
  team: (workspace: string, profile = "default") =>
    [...agentKeys.catalogs(), "@team", workspace, profile] as const,
  activityBatch: (
    items: readonly { name: string; workspaces: readonly string[] }[],
    days: number,
    profile: string
  ) => [...agentKeys.all, "activity-batch", profile, days, items] as const,
  activity: (name: string, days: number, workspace?: string | null, profile = "default") =>
    [...agentKeys.all, "activity", name, days, workspace ?? null, profile] as const,
  lists: () => [...agentKeys.all, "list"] as const,
  list: (workspace?: string | null, profile = "default") =>
    [...agentKeys.lists(), workspace ?? null, profile] as const,
  detail: (name: string, workspace?: string | null, profile = "default") =>
    [...agentKeys.all, "detail", name, workspace ?? null, profile] as const,
  soul: (name: string, workspace?: string | null, profile = "default") =>
    [...agentKeys.detail(name, workspace, profile), "soul"] as const,
  soulHistory: (name: string, workspace?: string | null, profile = "default") =>
    [...agentKeys.soul(name, workspace, profile), "history"] as const,
  memory: (name: string, workspace?: string | null, profile = "default") =>
    [...agentKeys.detail(name, workspace, profile), "memory"] as const,
  memoryHistory: (name: string, workspace?: string | null, profile = "default") =>
    [...agentKeys.memory(name, workspace, profile), "history"] as const,
  heartbeat: (name: string, workspace?: string | null, profile = "default") =>
    [...agentKeys.detail(name, workspace, profile), "heartbeat"] as const,
  /** The tools the agent sees in a scope; the Heartbeat page picks check-in writes from it. */
  tools: (name: string, workspace?: string | null, profile = "default") =>
    [...agentKeys.detail(name, workspace, profile), "tools"] as const,
  heartbeatHistory: (name: string, workspace?: string | null, profile = "default") =>
    [...agentKeys.heartbeat(name, workspace, profile), "history"] as const,
  heartbeatSources: (name: string, workspace?: string | null, profile = "default") =>
    [...agentKeys.heartbeat(name, workspace, profile), "sources"] as const,
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
