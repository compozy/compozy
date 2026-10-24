import type { InfiniteData, QueryClient } from "@tanstack/react-query";

import type { SessionPayload, SessionsResponse } from "../types";
import { sessionKeys } from "./query-keys";
import { sessionOpenPullRequestNumber } from "./session-pull-request";

function isSessionPayload(value: unknown, sessionId: string): value is SessionPayload {
  return typeof value === "object" && value !== null && (value as SessionPayload).id === sessionId;
}

function sessionFromListPages(value: unknown, sessionId: string): SessionPayload | undefined {
  const pages = (value as InfiniteData<SessionsResponse> | undefined)?.pages;
  if (!Array.isArray(pages)) return undefined;
  for (const page of pages) {
    const match = page?.sessions?.find(session => session.id === sessionId);
    if (match) return match;
  }
  return undefined;
}

/**
 * The open PR of a session already in the client's read model: its by-id reads
 * first, then any loaded catalog page. Edge-driven surfaces (the finished
 * toast) name a PR only when a read has reported it; they never fetch for it.
 */
export function cachedSessionOpenPullRequestNumber(
  queryClient: QueryClient,
  sessionId: string
): number | null {
  for (const [, data] of queryClient.getQueriesData({
    queryKey: sessionKeys.byIdRoot(sessionId),
  })) {
    if (isSessionPayload(data, sessionId)) return sessionOpenPullRequestNumber(data);
  }
  for (const [, data] of queryClient.getQueriesData({ queryKey: sessionKeys.lists() })) {
    const session = sessionFromListPages(data, sessionId);
    if (session) return sessionOpenPullRequestNumber(session);
  }
  return null;
}
