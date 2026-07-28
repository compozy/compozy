import type { FetchSessionEventsParams, SessionListFilters } from "../types";
import { normalizeSessionListFilters } from "./session-list-query";

function normalizedEventParams(params: FetchSessionEventsParams = {}): FetchSessionEventsParams {
  const normalized: FetchSessionEventsParams = {};
  const since = params.since?.trim();
  const type = params.type?.trim();
  const agentName = params.agent_name?.trim();
  const turnID = params.turn_id?.trim();
  if (since) normalized.since = since;
  if (params.limit !== undefined) normalized.limit = params.limit;
  if (params.after_sequence !== undefined) normalized.after_sequence = params.after_sequence;
  if (type) normalized.type = type;
  if (agentName) normalized.agent_name = agentName;
  if (turnID) normalized.turn_id = turnID;
  return normalized;
}

export const sessionKeys = {
  all: ["sessions"] as const,
  byId: (id: string) => [...sessionKeys.all, "by-id", id] as const,
  lists: () => [...sessionKeys.all, "list"] as const,
  workspaceLists: (workspace: string) => [...sessionKeys.lists(), workspace.trim()] as const,
  workspaceActivity: (workspace: string) =>
    [...sessionKeys.workspaceLists(workspace), "activity"] as const,
  list: (filters: SessionListFilters = {}) => {
    const normalized = normalizeSessionListFilters(filters);
    return [...sessionKeys.workspaceLists(normalized.workspace ?? ""), normalized] as const;
  },
  workspace: (workspace: string) => [...sessionKeys.all, "workspace", workspace] as const,
  detail: (workspace: string, id: string) =>
    [...sessionKeys.workspace(workspace), "detail", id] as const,
  events: (workspace: string, id: string) =>
    [...sessionKeys.detail(workspace, id), "events"] as const,
  eventsList: (workspace: string, id: string, params: FetchSessionEventsParams = {}) =>
    [...sessionKeys.events(workspace, id), normalizedEventParams(params)] as const,
  history: (workspace: string, id: string) =>
    [...sessionKeys.detail(workspace, id), "history"] as const,
  clearConversation: (workspace: string) =>
    [...sessionKeys.workspace(workspace), "clear-conversation"] as const,
  transcript: (workspace: string, id: string) =>
    [...sessionKeys.detail(workspace, id), "transcript"] as const,
  goal: (workspace: string, id: string) => [...sessionKeys.detail(workspace, id), "goal"] as const,
  clarifications: (workspace: string, id: string) =>
    [...sessionKeys.detail(workspace, id), "clarifications"] as const,
  recap: (workspace: string, id: string, limit?: number) =>
    [...sessionKeys.detail(workspace, id), "recap", limit ?? "default"] as const,
  ledger: (workspace: string, id: string) =>
    [...sessionKeys.detail(workspace, id), "ledger"] as const,
  usage: (workspace: string, id: string) =>
    [...sessionKeys.detail(workspace, id), "usage"] as const,
  toolArtifact: (workspace: string, artifactURI: string) =>
    [...sessionKeys.workspace(workspace), "tool-artifact", artifactURI] as const,
};
