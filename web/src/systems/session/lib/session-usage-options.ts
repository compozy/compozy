import { queryOptions } from "@tanstack/react-query";

import type { ComputerId } from "@/lib/computer-scope";

import { fetchSessionUsage, fetchSessionUsageTurns } from "../adapters/session-api";
import type { SessionState } from "../types";
import { sessionKeys } from "./query-keys";
import {
  SESSION_TRANSCRIPT_STALE_TIME_MS,
  SESSION_WARM_CACHE_POLICY,
} from "./session-query-policy";
import { isLiveSessionState, sessionStreamBackedRefetchInterval } from "./session-stream-liveness";

/**
 * Session token-usage aggregate. Usage accrues live as the agent reports it; the
 * stream's `session_usage_changed` re-reads it, and the stream-backed cadence covers a
 * session without a live stream. A stopped session is terminal and never refetches.
 */
export function sessionUsageOptions(
  workspace: string,
  id: string,
  sessionState?: SessionState | null,
  computerId?: ComputerId
) {
  return queryOptions({
    queryKey: sessionKeys.forComputer(computerId).usage(workspace, id),
    queryFn: ({ signal }) => fetchSessionUsage(workspace, id, signal, computerId),
    refetchInterval: isLiveSessionState(sessionState)
      ? () => sessionStreamBackedRefetchInterval(id, sessionState, computerId)
      : false,
    staleTime: SESSION_TRANSCRIPT_STALE_TIME_MS,
    ...SESSION_WARM_CACHE_POLICY,
    enabled: !!workspace && !!id,
  });
}

export function sessionUsageTurnsOptions(
  workspace: string,
  id: string,
  sessionState?: SessionState | null,
  computerId?: ComputerId
) {
  return queryOptions({
    queryKey: sessionKeys.forComputer(computerId).usageTurns(workspace, id),
    queryFn: ({ signal }) => fetchSessionUsageTurns(workspace, id, signal, computerId),
    refetchInterval: isLiveSessionState(sessionState)
      ? () => sessionStreamBackedRefetchInterval(id, sessionState, computerId)
      : false,
    staleTime: SESSION_TRANSCRIPT_STALE_TIME_MS,
    ...SESSION_WARM_CACHE_POLICY,
    enabled: !!workspace && !!id,
  });
}

/** The shared reset generation is client-owned and only changed by transcript fence resets. */
export function sessionContextResetOptions(
  workspaceId: string,
  sessionId: string,
  computerId?: ComputerId
) {
  return queryOptions({
    queryKey: sessionKeys.forComputer(computerId).contextReset(workspaceId, sessionId),
    queryFn: () => 0,
    enabled: false,
  });
}
