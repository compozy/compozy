/**
 * Query identity for calls and messages, co-located with the fetchers.
 *
 * Loaders, hooks, mutations, and the count probes all read these factories —
 * nothing redeclares a key or a fetcher at a consumer.
 *
 * Two conventions worth naming:
 *
 * - **Polling is opt-in per call site.** Calls have no SSE stream (the runtime
 *   emits server-side extension hooks, not browser events), so liveness is a
 *   poll. Surfaces pass `live` from `useCurrentWindowLiveDataEnabled()`, and a
 *   background window costs nothing.
 * - **Detail reads do not retry a refusal.** A 4xx from these routes is a
 *   decision — not found, wrong profile, not settled — and repeating it three
 *   times only delays the honest answer.
 */
import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import {
  countCalls,
  fetchCall,
  fetchCallPrompt,
  fetchCallResult,
  fetchCallSuperseded,
  listCallMessages,
  listCalls,
  type CallCountFilter,
  type CallMessagesFilter,
  type CallsListFilter,
} from "../adapters/agent-comms-api";
import { AgentCommsApiError } from "../adapters/agent-comms-api-error";
import { isTerminalCallState, toCallState } from "./call-state";
import { isScopeReady, type AgentCommsScope } from "./agent-comms-scope";
import { agentCommsKeys } from "./query-keys";

const LIST_STALE_TIME = 3_000;
export const LIVE_CALL_POLL_INTERVAL = 5_000;
const COUNT_STALE_TIME = 3_000;
const COUNT_POLL_INTERVAL = 10_000;
const DETAIL_RETRY_LIMIT = 2;

/**
 * Rows per page.
 *
 * The tree asks for the daemon ceiling so a whole delegation tree arrives in one
 * round trip and keyboard traversal never stalls on a page boundary. The panel
 * asks for far less on purpose: its pager is part of the contract, showing "25 of
 * 247 loaded" against a real total.
 */
export const CALLS_TREE_PAGE_SIZE = 200;
export const CALLS_PANEL_PAGE_SIZE = 25;

type CallsCursor = string | null;

function shouldRetryDetailQuery(failureCount: number, error: Error): boolean {
  if (error instanceof AgentCommsApiError && error.status >= 400 && error.status < 500) {
    return false;
  }
  return failureCount < DETAIL_RETRY_LIMIT;
}

function pollWhen(live: boolean, interval: number): number | false {
  return live ? interval : false;
}

/**
 * One filtered call population, paged by cursor.
 *
 * `total` rides on every page, so a consumer reading the first page already
 * holds the authoritative count for the whole population.
 */
export function callsListOptions(
  scope: AgentCommsScope,
  filter: CallsListFilter,
  live: boolean,
  enabled = true
) {
  const stableFilter: CallsListFilter = { ...filter, limit: filter.limit ?? CALLS_TREE_PAGE_SIZE };
  return infiniteQueryOptions({
    queryKey: agentCommsKeys.callsList(scope.workspaceId, scope.profileKey, stableFilter),
    queryFn: ({ pageParam, signal }) =>
      listCalls(
        scope.workspaceId,
        { ...stableFilter, ...(pageParam ? { cursor: pageParam } : {}) },
        scope.params,
        signal
      ),
    initialPageParam: null as CallsCursor,
    getNextPageParam: lastPage => lastPage.next_cursor ?? undefined,
    staleTime: LIST_STALE_TIME,
    refetchInterval: pollWhen(live, LIVE_CALL_POLL_INTERVAL),
    refetchOnWindowFocus: true,
    enabled: isScopeReady(scope) && enabled,
  });
}

/**
 * The summary projection: a counted read at `limit=1`.
 *
 * Keyed apart from the row pages on purpose — refreshing a badge must not evict
 * a page the operator is reading.
 */
export function callCountOptions(
  scope: AgentCommsScope,
  filter: CallCountFilter,
  live: boolean,
  enabled = true
) {
  return queryOptions({
    queryKey: agentCommsKeys.callCount(scope.workspaceId, scope.profileKey, filter),
    queryFn: ({ signal }) => countCalls(scope.workspaceId, filter, scope.params, signal),
    staleTime: COUNT_STALE_TIME,
    refetchInterval: pollWhen(live, COUNT_POLL_INTERVAL),
    refetchOnWindowFocus: true,
    enabled: isScopeReady(scope) && enabled,
  });
}

export function callDetailOptions(
  scope: AgentCommsScope,
  callId: string,
  live: boolean,
  enabled = true,
  pollUntilTerminal = false
) {
  return queryOptions({
    queryKey: agentCommsKeys.callDetail(scope.workspaceId, scope.profileKey, callId),
    queryFn: ({ signal }) => fetchCall(scope.workspaceId, callId, scope.params, signal),
    staleTime: LIST_STALE_TIME,
    refetchInterval: query => {
      const state = toCallState(query.state.data?.state);
      if (state !== null && isTerminalCallState(state)) return false;
      if (live) return LIVE_CALL_POLL_INTERVAL;
      if (!pollUntilTerminal || query.state.status === "error" || query.state.error !== null) {
        return false;
      }
      return LIVE_CALL_POLL_INTERVAL;
    },
    retry: shouldRetryDetailQuery,
    enabled: isScopeReady(scope) && Boolean(callId) && enabled,
  });
}

/**
 * The whole stored payload, fetched only when the operator asks for it.
 *
 * Never enabled by default: list and detail already carry `result_preview`, and
 * a payload can run to megabytes. A read before the call settles answers `409
 * call_not_settled`, which the caller shows rather than retrying.
 */
export function callResultOptions(scope: AgentCommsScope, callId: string, enabled = false) {
  return queryOptions({
    queryKey: agentCommsKeys.callResult(scope.workspaceId, scope.profileKey, callId),
    queryFn: ({ signal }) => fetchCallResult(scope.workspaceId, callId, scope.params, signal),
    staleTime: Number.POSITIVE_INFINITY,
    refetchOnWindowFocus: false,
    retry: shouldRetryDetailQuery,
    enabled: isScopeReady(scope) && Boolean(callId) && enabled,
  });
}

/**
 * The whole ask, fetched only when the operator asks for it.
 *
 * A settled call's prompt never changes, so once read it is cached forever. Two
 * surfaces want it: the bounded-preview disclosure, and Call again, which must
 * repeat the *exact* ask rather than a truncated preview of it.
 */
export function callPromptOptions(scope: AgentCommsScope, callId: string, enabled = false) {
  return queryOptions({
    queryKey: agentCommsKeys.callPrompt(scope.workspaceId, scope.profileKey, callId),
    queryFn: ({ signal }) => fetchCallPrompt(scope.workspaceId, callId, scope.params, signal),
    staleTime: Number.POSITIVE_INFINITY,
    refetchOnWindowFocus: false,
    retry: shouldRetryDetailQuery,
    enabled: isScopeReady(scope) && Boolean(callId) && enabled,
  });
}

/**
 * A result that landed after the call had already settled.
 *
 * Evidence, never a state change — and only ever fetched on request, since the
 * record already carries a bounded preview of it.
 */
export function callSupersededOptions(scope: AgentCommsScope, callId: string, enabled = false) {
  return queryOptions({
    queryKey: agentCommsKeys.callSuperseded(scope.workspaceId, scope.profileKey, callId),
    queryFn: ({ signal }) => fetchCallSuperseded(scope.workspaceId, callId, scope.params, signal),
    staleTime: Number.POSITIVE_INFINITY,
    refetchOnWindowFocus: false,
    retry: shouldRetryDetailQuery,
    enabled: isScopeReady(scope) && Boolean(callId) && enabled,
  });
}

/**
 * One session's mailbox, paged by cursor.
 *
 * Uncounted by contract — `MessagePage` carries no total — so consumers show
 * what is loaded and whether more exists, never a substituted page length.
 */
export function callMessagesOptions(
  scope: AgentCommsScope,
  filter: CallMessagesFilter,
  live: boolean,
  enabled = true
) {
  const stableFilter: CallMessagesFilter = {
    ...filter,
    limit: filter.limit ?? CALLS_PANEL_PAGE_SIZE,
  };
  return infiniteQueryOptions({
    queryKey: agentCommsKeys.messagesList(scope.workspaceId, scope.profileKey, stableFilter),
    queryFn: ({ pageParam, signal }) =>
      listCallMessages(
        scope.workspaceId,
        { ...stableFilter, ...(pageParam ? { cursor: pageParam } : {}) },
        scope.params,
        signal
      ),
    initialPageParam: null as CallsCursor,
    getNextPageParam: lastPage => lastPage.next_cursor ?? undefined,
    staleTime: LIST_STALE_TIME,
    refetchInterval: pollWhen(live, LIVE_CALL_POLL_INTERVAL),
    refetchOnWindowFocus: true,
    enabled: isScopeReady(scope) && enabled,
  });
}
