import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import {
  fetchSession,
  fetchSessionClarifications,
  fetchSessionEvents,
  fetchSessionHistory,
  fetchSessionInputs,
  fetchSessionGoal,
  fetchSessionLedger,
  fetchSessionRecap,
  fetchSessionTranscript,
  fetchSessionUsage,
  fetchSessions,
  SessionLedgerUnavailableError,
} from "../adapters/session-api";
import { fetchSessionCommands } from "../adapters/session-command-api";
import { fetchSessionOwner } from "../adapters/session-owner-api";
import { fetchToolArtifactPage } from "../adapters/tool-artifact-api";
import type { FetchSessionEventsParams } from "../adapters/session-api";
import type { SessionListFilters, SessionState } from "../types";
import { sessionKeys } from "./query-keys";
import { normalizeSessionListFilters, sessionListRequest } from "./session-list-query";
import {
  nextTranscriptPageParam,
  transcriptPageFromResponse,
  transcriptPageMatchesFence,
  transcriptPageRequest,
  type SessionTranscriptPageParam,
} from "./session-transcript-query";

const SESSION_LIVE_REFETCH_INTERVAL_MS = 5_000;
const SESSION_STARTING_REFETCH_INTERVAL_MS = 500;
const SESSION_DETAIL_STALE_TIME_MS = 2_000;
export const SESSION_OWNER_STALE_TIME_MS = 30_000;
const SESSION_TRANSCRIPT_STALE_TIME_MS = 10_000;
const SESSION_WARM_CACHE_GC_TIME_MS = 30 * 60 * 1_000;
const TOOL_ARTIFACT_PAGE_BYTES = 64 * 1_024;

/**
 * The only workspace-agnostic session key. Owner projections select a workspace but never carry
 * rendered session data, so they remain separate from the workspace-prefixed `sessionKeys`.
 */
export const sessionOwnerKeys = {
  all: ["session-owner"] as const,
  detail: (sessionId: string) => [...sessionOwnerKeys.all, sessionId] as const,
};

export interface SessionOwnerDialogState {
  sessionId: string;
  workspaceId: string;
  workspaceName: string;
}

export function sessionOwnerOptions(sessionId: string) {
  return queryOptions({
    queryKey: sessionOwnerKeys.detail(sessionId),
    queryFn: ({ signal }) => fetchSessionOwner(sessionId, signal),
    staleTime: SESSION_OWNER_STALE_TIME_MS,
    enabled: !!sessionId,
  });
}

/**
 * Session detail + transcript are the hot return path for `/agents/:name/sessions/:id`.
 * Keep them inactive for 30 minutes so tab restores and cross-route returns render from
 * cache immediately. Session detail uses bounded live polling, while transcript freshness is
 * driven by its SSE tail and explicit recovery reads.
 */
const SESSION_WARM_CACHE_POLICY = {
  gcTime: SESSION_WARM_CACHE_GC_TIME_MS,
} as const;

/**
 * Live session states worth polling: while a session is `active|starting|stopping`, detail and
 * usage aggregates can still change. A `stopped` session is terminal; no polling.
 */
export function isLiveSessionState(state: SessionState | null | undefined): boolean {
  return state === "active" || state === "starting" || state === "stopping";
}

export function sessionsListOptions(filters: SessionListFilters = {}) {
  const normalizedFilters = normalizeSessionListFilters(filters);
  return infiniteQueryOptions({
    queryKey: sessionKeys.list(normalizedFilters),
    queryFn: ({ pageParam, signal }) =>
      fetchSessions(sessionListRequest(normalizedFilters, pageParam), signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: lastPage => (lastPage.page.has_more ? lastPage.page.next_cursor : undefined),
    staleTime: 2_000,
  });
}

export function sessionDetailOptions(workspace: string, id: string) {
  return queryOptions({
    queryKey: sessionKeys.detail(workspace, id),
    queryFn: ({ signal }) => fetchSession(workspace, id, signal),
    refetchInterval: query => {
      const state = query.state.data?.state;
      if (state === "starting") return SESSION_STARTING_REFETCH_INTERVAL_MS;
      return isLiveSessionState(state) ? SESSION_LIVE_REFETCH_INTERVAL_MS : false;
    },
    staleTime: SESSION_DETAIL_STALE_TIME_MS,
    ...SESSION_WARM_CACHE_POLICY,
    enabled: !!workspace && !!id,
  });
}

export function sessionEventsOptions(
  workspace: string,
  id: string,
  params?: FetchSessionEventsParams
) {
  return queryOptions({
    queryKey: sessionKeys.eventsList(workspace, id, params),
    queryFn: ({ signal }) => fetchSessionEvents(workspace, id, params, signal),
    staleTime: 5_000,
    enabled: !!workspace && !!id,
  });
}

export function sessionHistoryOptions(workspace: string, id: string) {
  return queryOptions({
    queryKey: sessionKeys.history(workspace, id),
    queryFn: ({ signal }) => fetchSessionHistory(workspace, id, signal),
    staleTime: 10_000,
    enabled: !!workspace && !!id,
  });
}

/**
 * Canonical command projection for the current session. SSE frames invalidate this exact key and
 * the composer refetches it whenever the native trigger popover opens.
 */
export function sessionCommandsOptions(workspace: string, id: string) {
  const workspaceId = workspace.trim();
  const sessionId = id.trim();
  return queryOptions({
    queryKey: sessionKeys.commands(workspaceId, sessionId),
    queryFn: ({ signal }) => fetchSessionCommands(workspaceId, sessionId, signal),
    staleTime: 30_000,
    enabled: workspaceId.length > 0 && sessionId.length > 0,
  });
}

export function sessionGoalOptions(workspace: string, id: string) {
  return queryOptions({
    queryKey: sessionKeys.goal(workspace, id),
    queryFn: ({ signal }) => fetchSessionGoal(workspace, id, signal),
    staleTime: Number.POSITIVE_INFINITY,
    enabled: !!workspace && !!id,
  });
}

/**
 * Live pending clarification for one session — the exact authority for pending truth. Never polls:
 * the transcript `clarify` SSE event wakes it via explicit invalidation (see `use-session-live-tail`),
 * and a successful answer reconciles the exact key. A short stale window lets focus/remount self-heal
 * a dropped wake.
 */
export function sessionClarificationsOptions(workspace: string, id: string, enabled = true) {
  return queryOptions({
    queryKey: sessionKeys.clarifications(workspace, id),
    queryFn: ({ signal }) => fetchSessionClarifications(workspace, id, signal),
    staleTime: 5_000,
    enabled: !!workspace && !!id && enabled,
  });
}

/** Exact current-generation pending operator input, owned by the daemon. */
export function sessionInputsOptions(workspace: string, id: string, enabled = true) {
  return queryOptions({
    queryKey: sessionKeys.inputQueue(workspace, id),
    queryFn: ({ signal }) => fetchSessionInputs(workspace, id, signal),
    refetchInterval: SESSION_LIVE_REFETCH_INTERVAL_MS,
    staleTime: 1_000,
    enabled: !!workspace && !!id && enabled,
  });
}

export function sessionTranscriptOptions(workspace: string, id: string) {
  return infiniteQueryOptions({
    queryKey: sessionKeys.transcript(workspace, id),
    queryFn: async ({ pageParam, signal }) => {
      const response = await fetchSessionTranscript(
        workspace,
        id,
        transcriptPageRequest(pageParam),
        signal
      );
      if (!transcriptPageMatchesFence(response, pageParam)) {
        throw new Error("Session transcript changed while loading older messages");
      }
      return transcriptPageFromResponse(response);
    },
    initialPageParam: undefined as SessionTranscriptPageParam,
    getNextPageParam: nextTranscriptPageParam,
    staleTime: SESSION_TRANSCRIPT_STALE_TIME_MS,
    ...SESSION_WARM_CACHE_POLICY,
    enabled: !!workspace && !!id,
  });
}

export function sessionToolArtifactOptions(
  workspace: string,
  artifactURI: string,
  enabled: boolean
) {
  return infiniteQueryOptions({
    queryKey: sessionKeys.toolArtifact(workspace, artifactURI),
    queryFn: ({ pageParam, signal }) =>
      fetchToolArtifactPage(workspace, artifactURI, pageParam, TOOL_ARTIFACT_PAGE_BYTES, signal),
    initialPageParam: 0,
    getNextPageParam: lastPage => (lastPage.eof ? undefined : lastPage.next_offset),
    staleTime: Number.POSITIVE_INFINITY,
    ...SESSION_WARM_CACHE_POLICY,
    enabled: enabled && !!workspace && !!artifactURI,
    retry: false,
  });
}

export function sessionRecapOptions(workspace: string, id: string, limit?: number) {
  return queryOptions({
    queryKey: sessionKeys.recap(workspace, id, limit),
    queryFn: ({ signal }) => fetchSessionRecap(workspace, id, limit, signal),
    staleTime: 10_000,
    enabled: !!workspace && !!id,
  });
}

/**
 * Session token-usage aggregate. Usage accrues live as the agent reports it, so
 * poll on the same bounded cadence as the transcript while the session is live;
 * a stopped session is terminal and never refetches.
 */
export function sessionUsageOptions(
  workspace: string,
  id: string,
  sessionState?: SessionState | null
) {
  return queryOptions({
    queryKey: sessionKeys.usage(workspace, id),
    queryFn: ({ signal }) => fetchSessionUsage(workspace, id, signal),
    refetchInterval: isLiveSessionState(sessionState) ? SESSION_LIVE_REFETCH_INTERVAL_MS : false,
    staleTime: SESSION_TRANSCRIPT_STALE_TIME_MS,
    ...SESSION_WARM_CACHE_POLICY,
    enabled: !!workspace && !!id,
  });
}

export interface SessionLedgerQueryOptions {
  enabled?: boolean;
}

export function sessionLedgerOptions(
  workspace: string,
  id: string,
  options?: SessionLedgerQueryOptions
) {
  const enabled = (options?.enabled ?? true) && !!workspace && !!id;
  return queryOptions({
    queryKey: sessionKeys.ledger(workspace, id),
    queryFn: ({ signal }) => fetchSessionLedger(workspace, id, signal),
    staleTime: 10_000,
    enabled,
    retry: (failureCount, error) => {
      if (error instanceof SessionLedgerUnavailableError) {
        return false;
      }
      return failureCount < 1;
    },
  });
}
