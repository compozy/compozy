import { computerQueryKey, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import { normalizeAgentCatalogFilter } from "./agent-catalog-query";
import type { AgentCatalogStableFilter } from "../types";

export interface AgentHeartbeatStatusKeyOptions {
  profile?: string;
  workspaceId?: string | null;
  sessionId?: string | null;
  includeSessionHealth?: boolean;
  includeRecentWakeEvents?: boolean;
}

export function agentKeysFor(computerId: ComputerId = LOCAL_COMPUTER_ID) {
  const all = computerQueryKey(computerId, "agents");
  const catalogs = () => [...all, "catalog"] as const;
  const catalogByWorkspace = (workspace: string) => [...catalogs(), workspace.trim()] as const;
  const aggregates = () => [...catalogs(), "@aggregate"] as const;
  const instances = () => [...all, "instances"] as const;
  const instanceLists = () => [...instances(), "list"] as const;
  const instanceCheckIns = (id: string) => [...all, "instance-check-ins", id] as const;
  const instanceReports = (id: string) => [...all, "instance-reports", id] as const;
  const skillReviews = () => [...all, "skill-review"] as const;
  const lists = () => [...all, "list"] as const;
  const detail = (name: string, workspace?: string | null, profile = "default") =>
    [...all, "detail", name, workspace ?? null, profile] as const;
  const soul = (name: string, workspace?: string | null, profile = "default") =>
    [...detail(name, workspace, profile), "soul"] as const;
  const memory = (name: string, workspace?: string | null, profile = "default") =>
    [...detail(name, workspace, profile), "memory"] as const;
  const heartbeat = (name: string, workspace?: string | null, profile = "default") =>
    [...detail(name, workspace, profile), "heartbeat"] as const;
  const heartbeatStatuses = (name: string, workspace?: string | null, profile = "default") =>
    [...heartbeat(name, workspace, profile), "status"] as const;

  return {
    all,
    catalogs,
    catalogByWorkspace,
    catalog: (workspace: string, filters: AgentCatalogStableFilter = {}) =>
      [...catalogByWorkspace(workspace), normalizeAgentCatalogFilter(filters)] as const,
    aggregates,
    aggregate: (workspaceIds: readonly string[], filters: AgentCatalogStableFilter = {}) =>
      [
        ...aggregates(),
        [...workspaceIds].sort().join(","),
        normalizeAgentCatalogFilter(filters),
      ] as const,
    instances,
    instanceLists,
    instanceList: (view: string, profiles: readonly string[]) =>
      [...instanceLists(), view, profiles.join(",")] as const,
    instanceAway: (id: string) => [...instances(), id, "away"] as const,
    instanceCheckIns,
    instanceDayCheckIns: (id: string) => [...instanceCheckIns(id), "day"] as const,
    instanceOccurrence: (id: string, occurrence: string) =>
      [...all, "instance-occurrence", id, occurrence] as const,
    instanceReports,
    skillReviews,
    skillReview: (id: string, profile = "") => [...skillReviews(), id, profile] as const,
    instanceIndeterminateReports: (id: string) => [...instanceReports(id), "indeterminate"] as const,
    instanceProjectedReports: (id: string) => [...instanceReports(id), "projected"] as const,
    instanceOccurrenceCheckIn: (id: string, occurrence: string) =>
      [...instanceCheckIns(id), "occurrence", occurrence] as const,
    alwaysOnCatalog: (workspace: string, profile = "default") =>
      [...catalogs(), "@always-on", workspace, profile] as const,
    team: (workspace: string, profile = "default") =>
      [...catalogs(), "@team", workspace, profile] as const,
    activityBatch: (
      items: readonly { name: string; workspaces: readonly string[] }[],
      days: number,
      profile: string
    ) => [...all, "activity-batch", profile, days, items] as const,
    activity: (name: string, days: number, workspace?: string | null, profile = "default") =>
      [...all, "activity", name, days, workspace ?? null, profile] as const,
    lists,
    list: (workspace?: string | null, profile = "default") =>
      [...lists(), workspace ?? null, profile] as const,
    detail,
    soul,
    soulHistory: (name: string, workspace?: string | null, profile = "default") =>
      [...soul(name, workspace, profile), "history"] as const,
    memory,
    memoryHistory: (name: string, workspace?: string | null, profile = "default") =>
      [...memory(name, workspace, profile), "history"] as const,
    heartbeat,
    tools: (name: string, workspace?: string | null, profile = "default") =>
      [...detail(name, workspace, profile), "tools"] as const,
    heartbeatHistory: (name: string, workspace?: string | null, profile = "default") =>
      [...heartbeat(name, workspace, profile), "history"] as const,
    heartbeatSources: (name: string, workspace?: string | null, profile = "default") =>
      [...heartbeat(name, workspace, profile), "sources"] as const,
    heartbeatStatuses,
    heartbeatStatus: (name: string, options: AgentHeartbeatStatusKeyOptions = {}) =>
      [
        ...heartbeatStatuses(name, options.workspaceId, options.profile),
        options.sessionId ?? null,
        options.includeSessionHealth ?? null,
        options.includeRecentWakeEvents ?? null,
      ] as const,
  };
}

export const agentKeys = {
  ...agentKeysFor(LOCAL_COMPUTER_ID),
  forComputer: agentKeysFor,
};
