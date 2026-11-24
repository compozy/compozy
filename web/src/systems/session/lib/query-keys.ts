import {
  computerQueryKey,
  isRemoteComputerId,
  LOCAL_COMPUTER_ID,
  type ComputerId,
} from "@/lib/computer-scope";
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

export function sessionKeysFor(computerId: ComputerId = LOCAL_COMPUTER_ID) {
  const all = computerQueryKey(computerId, "sessions");
  const commandsRoot = computerQueryKey(computerId, "session-commands");
  const lists = () => [...all, "list"] as const;
  const workspaceLists = (workspace: string) => [...lists(), workspace.trim()] as const;
  const workspaceCommands = (workspace: string) => [...commandsRoot, workspace.trim()] as const;
  const historyCleanup = () => [...all, "history-cleanup"] as const;
  const workspaceLayouts = () => [...all, "workspace-layout"] as const;
  const workspace = (ws: string) => [...all, "workspace", ws] as const;
  const byIdAll = () => [...all, "by-id"] as const;
  const byIdRoot = (id: string) => [...byIdAll(), id.trim()] as const;
  const detail = (ws: string, id: string) => [...workspace(ws), "detail", id] as const;
  const events = (ws: string, id: string) => [...detail(ws, id), "events"] as const;
  const transcriptSearchRoot = (ws: string, id: string) =>
    [...detail(ws, id), "transcript-search"] as const;
  const fileSearchRoot = (ws: string, id: string) => [...detail(ws, id), "files"] as const;
  const subagents = (ws: string, id: string) => [...detail(ws, id), "subagents"] as const;
  const sides = (ws: string, id: string) => [...detail(ws, id), "sides"] as const;
  const processes = (ws: string, id: string) => [...detail(ws, id), "processes"] as const;
  const linkCandidatesRoot = (ws: string, id: string) =>
    [...detail(ws, id), "link-candidates"] as const;

  return {
    all,
    deletions: () => [...all, "delete"] as const,
    archives: () => [...all, "archive"] as const,
    commandsRoot,
    workspaceCommands,
    commands: (ws: string, id: string) => [...workspaceCommands(ws), id.trim()] as const,
    lists,
    /** Exact cross-workspace attention counts — operator scope, no workspace segment. */
    attentionSummary: () => [...all, "attention-summary"] as const,
    /**
     * The catalog stream's connection status for one stream scope, written by the stream
     * and read by the polls it covers. The stream URL carries the profile scope.
     */
    catalogStreamStatus: (streamUrl: string) =>
      [...all, "catalog-stream-status", streamUrl] as const,
    /** Processes left running by stopped, archived, or unknown sessions — operator scope. */
    processesLeftRunning: () => [...all, "processes-left-running"] as const,
    /** Archived-history cleanup (#1717): one profile's run, and the dry runs the dialog previews. */
    historyCleanup,
    historyCleanupRun: (profile: string) => [...historyCleanup(), "run", profile.trim()] as const,
    historyCleanupPreview: (profile: string, request: unknown) =>
      [...historyCleanup(), "preview", profile.trim(), request] as const,
    workspaceLists,
    catalogPage: (filters: SessionListFilters) =>
      [...workspaceLists(filters.workspace_id ?? ""), "page", filters] as const,
    launchPreview: (filters: SessionListFilters) => [...all, "launch-preview", filters] as const,
    /** One profile's session folders in a workspace; under the list family so catalog wakes reread it. */
    folders: (ws: string, profile: string) =>
      [...workspaceLists(ws), "folders", profile.trim()] as const,
    /** Every profile's project folders (issue #1044); a layout change rereads them all. */
    workspaceLayouts,
    /** One profile's project folders and project order in All projects. */
    workspaceLayout: (profile: string) => [...workspaceLayouts(), profile.trim()] as const,
    workspaceActivity: (ws: string) => [...workspaceLists(ws), "activity"] as const,
    list: (filters: SessionListFilters = {}) => {
      const normalized = normalizeSessionListFilters(filters);
      return [...workspaceLists(normalized.workspace_id ?? ""), normalized] as const;
    },
    workspace,
    byIdAll,
    byIdRoot,
    byId: (id: string, profileKey: string) => [...byIdRoot(id), profileKey.trim()] as const,
    descendants: (id: string, profile: string) =>
      [...byIdRoot(id), "descendants", profile.trim()] as const,
    detail,
    events,
    eventsList: (ws: string, id: string, params: FetchSessionEventsParams = {}) =>
      [...events(ws, id), normalizedEventParams(params)] as const,
    history: (ws: string, id: string) => [...detail(ws, id), "history"] as const,
    clearConversation: (ws: string) => [...workspace(ws), "clear-conversation"] as const,
    rewindConversation: (ws: string) => [...workspace(ws), "rewind-conversation"] as const,
    transcript: (ws: string, id: string) => [...detail(ws, id), "transcript"] as const,
    /** A derived child's source transcript through the derive cut (#1885). */
    inheritedTranscript: (ws: string, id: string) =>
      [...detail(ws, id), "inherited-transcript"] as const,
    /** Operator-message outline over full retained history (S9). */
    transcriptOutline: (ws: string, id: string) =>
      [...detail(ws, id), "transcript-outline"] as const,
    /** Every committed full-history search of one session (S8). */
    transcriptSearchRoot,
    transcriptSearch: (ws: string, id: string, query: string, limit: number) =>
      [...transcriptSearchRoot(ws, id), { limit, q: query }] as const,
    /** Mention-picker file search over the session's working directory; one entry per query. */
    fileSearchRoot,
    fileSearch: (ws: string, id: string, query: string, limit: number) =>
      [...fileSearchRoot(ws, id), { limit, q: query }] as const,
    goal: (ws: string, id: string) => [...detail(ws, id), "goal"] as const,
    clarifications: (ws: string, id: string) => [...detail(ws, id), "clarifications"] as const,
    /** Restart-durable interaction records read by one explicit daemon status. */
    interactions: (ws: string, id: string, status: string) =>
      [...detail(ws, id), "interactions", status] as const,
    inputQueue: (ws: string, id: string) => [...detail(ws, id), "input-queue"] as const,
    recap: (ws: string, id: string, limit?: number) =>
      [...detail(ws, id), "recap", limit ?? "default"] as const,
    usage: (ws: string, id: string) => [...detail(ws, id), "usage"] as const,
    usageTurns: (ws: string, id: string) => [...detail(ws, id), "usage-turns"] as const,
    /** What a continue/fork would carry; `""` is the whole-session cut. */
    derivePreview: (ws: string, id: string, messageId = "") =>
      [...detail(ws, id), "derive-preview", messageId.trim()] as const,
    /** Client-held roster the session stream writes (`subagents_snapshot` / `subagent_updated`). */
    subagentRoster: (ws: string, id: string) => [...detail(ws, id), "subagent-roster"] as const,
    /** The bounded mid-prompt read that feeds the roster while the session SSE is closed. */
    subagentRosterPoll: (ws: string, id: string) =>
      [...detail(ws, id), "subagent-roster-poll"] as const,
    /** A parent's direct subagents, every page of the list route (S10, chip preview). */
    subagents,
    /** Short pages of the same list for a session row's subagent branch; parent invalidation reaches it. */
    subagentBranch: (ws: string, id: string) => [...subagents(ws, id), "branch"] as const,
    /** A session's side chats; `session_side_changed` for this parent rereads every variant. */
    sides,
    sidesList: (ws: string, id: string, archived: boolean) =>
      [...sides(ws, id), archived ? "archived" : "open"] as const,
    /** The session's daemon-owned monitors (composer monitor popover). */
    monitors: (ws: string, id: string) => [...detail(ws, id), "monitors"] as const,
    /** What MCP servers this session can use (composer MCP popover). */
    mcp: (ws: string, id: string) => [...detail(ws, id), "mcp"] as const,
    /** Processes the session started that are still running (composer processes popover). */
    processes,
    processesForProfile: (ws: string, id: string, profile: string) =>
      [...processes(ws, id), profile] as const,
    /** Everything linked to the session: under `detail`, so a detail sweep reaches it. */
    linkedWork: (ws: string, id: string) => [...detail(ws, id), "linked-work"] as const,
    /** Server pages of what this session could link, one entry per search. */
    linkCandidatesRoot,
    linkCandidates: (ws: string, id: string, kind: string, query: string) =>
      [...linkCandidatesRoot(ws, id), { kind, q: query }] as const,
    contextReset: (ws: string, id: string) => [...detail(ws, id), "context-reset"] as const,
    toolArtifact: (ws: string, artifactURI: string) =>
      [...workspace(ws), "tool-artifact", artifactURI] as const,
  };
}

const localSessionKeys = sessionKeysFor();

export const sessionKeys = {
  ...localSessionKeys,
  forComputer: (computerId?: ComputerId) =>
    isRemoteComputerId(computerId) ? sessionKeysFor(computerId) : localSessionKeys,
};
