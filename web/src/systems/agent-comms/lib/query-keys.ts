/**
 * Cache identity for calls and messages.
 *
 * Two rules shape every key here:
 *
 * 1. **Scope is in the key, never in a filter.** Workspace and profile are
 *    segments, so a workspace switch or a profile switch can never read another
 *    scope's rows out of cache.
 * 2. **Counts key separately from rows.** A count probe and a row page describe
 *    the same population but answer different questions; giving them separate
 *    entries means refreshing a badge never evicts a page the operator is
 *    reading, and vice versa.
 *
 * Filter segments carry the contract's own param names in a fixed order, so a
 * key read in the devtools lines up with the request that produced it.
 */
import type { CallMessagesFilter, CallsListFilter } from "../adapters/agent-comms-api";

function normalizeText(value?: string | null) {
  return value?.trim() ?? "";
}

function normalizeLimit(value?: number | null) {
  return value ?? 0;
}

function normalizeFlag(value?: boolean) {
  return value === true;
}

/**
 * The population segments, in contract order. `cursor` is deliberately absent:
 * continuation belongs in `pageParam`, so every page of one filtered population
 * shares a cache entry.
 */
function callFilterSegments(filter?: CallsListFilter) {
  return [
    normalizeText(filter?.state),
    // `attention` selects the *unresolved* subset, which a state filter cannot
    // express — the two populations must never share a cache entry.
    normalizeFlag(filter?.attention),
    normalizeText(filter?.caller),
    normalizeText(filter?.child_session_id),
    normalizeText(filter?.root_session_id),
    normalizeText(filter?.agent),
    normalizeLimit(filter?.limit),
  ] as const;
}

/** A count has no page, so its segments stop at the population. */
function callCountSegments(filter?: Omit<CallsListFilter, "cursor" | "limit">) {
  return [
    normalizeText(filter?.state),
    normalizeFlag(filter?.attention),
    normalizeText(filter?.caller),
    normalizeText(filter?.child_session_id),
    normalizeText(filter?.root_session_id),
    normalizeText(filter?.agent),
  ] as const;
}

function messageFilterSegments(filter?: CallMessagesFilter) {
  return [normalizeText(filter?.session), normalizeLimit(filter?.limit)] as const;
}

export const agentCommsKeys = {
  all: ["agent-comms"] as const,

  /** Everything one workspace owns, under one profile view. */
  scope: (workspaceId: string, profileKey: string) =>
    [
      ...agentCommsKeys.all,
      "workspace",
      normalizeText(workspaceId),
      "profile",
      normalizeText(profileKey),
    ] as const,

  callsRoot: (workspaceId: string, profileKey: string) =>
    [...agentCommsKeys.scope(workspaceId, profileKey), "calls"] as const,
  callsList: (workspaceId: string, profileKey: string, filter?: CallsListFilter) =>
    [
      ...agentCommsKeys.callsRoot(workspaceId, profileKey),
      "list",
      ...callFilterSegments(filter),
    ] as const,

  /** Count probes: `limit=1` reads whose only product is `total`. */
  callCountsRoot: (workspaceId: string, profileKey: string) =>
    [...agentCommsKeys.callsRoot(workspaceId, profileKey), "count"] as const,
  callCount: (
    workspaceId: string,
    profileKey: string,
    filter?: Omit<CallsListFilter, "cursor" | "limit">
  ) =>
    [
      ...agentCommsKeys.callCountsRoot(workspaceId, profileKey),
      ...callCountSegments(filter),
    ] as const,

  /**
   * One call and its attachments.
   *
   * Workspace-segmented like every other population, because the daemon derives
   * a call's scope from the route: the same id read under a different workspace
   * is a different request with a different answer, and sharing a cache entry
   * across workspaces would serve one workspace's record to another.
   */
  callDetails: (workspaceId: string, profileKey: string) =>
    [...agentCommsKeys.scope(workspaceId, profileKey), "call"] as const,
  callDetail: (workspaceId: string, profileKey: string, callId: string) =>
    [
      ...agentCommsKeys.callDetails(workspaceId, profileKey),
      "detail",
      normalizeText(callId),
    ] as const,
  callResult: (workspaceId: string, profileKey: string, callId: string) =>
    [
      ...agentCommsKeys.callDetails(workspaceId, profileKey),
      "result",
      normalizeText(callId),
    ] as const,
  callPrompt: (workspaceId: string, profileKey: string, callId: string) =>
    [
      ...agentCommsKeys.callDetails(workspaceId, profileKey),
      "prompt",
      normalizeText(callId),
    ] as const,
  callSuperseded: (workspaceId: string, profileKey: string, callId: string) =>
    [
      ...agentCommsKeys.callDetails(workspaceId, profileKey),
      "superseded",
      normalizeText(callId),
    ] as const,

  messagesRoot: (workspaceId: string, profileKey: string) =>
    [...agentCommsKeys.scope(workspaceId, profileKey), "messages"] as const,
  messagesList: (workspaceId: string, profileKey: string, filter?: CallMessagesFilter) =>
    [
      ...agentCommsKeys.messagesRoot(workspaceId, profileKey),
      "list",
      ...messageFilterSegments(filter),
    ] as const,
};
