import { queryOptions } from "@tanstack/react-query";

import {
  getSessionChanges,
  listSessionChangeBases,
  type SessionChangesRequest,
} from "../adapters/session-changes-api";

export const sessionChangesKeys = {
  all: ["session-changes"] as const,
  session: (workspaceId: string, sessionId: string) =>
    [...sessionChangesKeys.all, workspaceId, sessionId] as const,
  summary: (request: SessionChangesRequest) =>
    [
      ...sessionChangesKeys.session(request.workspaceId, request.sessionId),
      "summary",
      request.scope ?? "default",
      request.base ?? "",
    ] as const,
  bases: (workspaceId: string, sessionId: string, query: string) =>
    [...sessionChangesKeys.session(workspaceId, sessionId), "bases", query] as const,
};

/**
 * The Changes summary. A refetch keeps the last good summary on screen, so a failed refresh
 * reads "Couldn't refresh · Retry" over it (US-010.EC-2); focusing the window refreshes it.
 */
export function sessionChangesOptions(request: SessionChangesRequest) {
  return queryOptions({
    queryKey: sessionChangesKeys.summary(request),
    queryFn: ({ signal }) => getSessionChanges(request, signal),
    staleTime: 1_000,
    retry: false,
    refetchOnWindowFocus: true,
  });
}

export function sessionChangeBasesOptions(workspaceId: string, sessionId: string, query: string) {
  return queryOptions({
    queryKey: sessionChangesKeys.bases(workspaceId, sessionId, query),
    queryFn: ({ signal }) => listSessionChangeBases({ workspaceId, sessionId, query }, signal),
    staleTime: 30_000,
    retry: false,
  });
}
