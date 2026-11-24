import { queryOptions } from "@tanstack/react-query";

import { computerQueryKey, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import {
  getSessionChanges,
  listSessionChangeBases,
  type SessionChangesRequest,
} from "../adapters/session-changes-api";

export function sessionChangesKeysFor(computerId: ComputerId = LOCAL_COMPUTER_ID) {
  const all = computerQueryKey(computerId, "session-changes");
  const session = (workspaceId: string, sessionId: string) =>
    [...all, workspaceId, sessionId] as const;
  return {
    all,
    session,
    summary: (request: SessionChangesRequest) =>
      [
        ...session(request.workspaceId, request.sessionId),
        "summary",
        request.scope ?? "default",
        request.base ?? "",
      ] as const,
    bases: (workspaceId: string, sessionId: string, query: string) =>
      [...session(workspaceId, sessionId), "bases", query] as const,
  };
}

export const sessionChangesKeys = {
  ...sessionChangesKeysFor(LOCAL_COMPUTER_ID),
  forComputer: sessionChangesKeysFor,
};

/**
 * The Changes summary. A refetch keeps the last good summary on screen, so a failed refresh
 * reads "Couldn't refresh · Retry" over it (US-010.EC-2); focusing the window refreshes it.
 */
export function sessionChangesOptions(request: SessionChangesRequest) {
  const keys = sessionChangesKeys.forComputer(request.computerId);
  return queryOptions({
    queryKey: keys.summary(request),
    queryFn: ({ signal }) => getSessionChanges(request, signal),
    staleTime: 1_000,
    retry: false,
    refetchOnWindowFocus: true,
  });
}

export function sessionChangeBasesOptions(
  workspaceId: string,
  sessionId: string,
  query: string,
  computerId?: ComputerId
) {
  const keys = sessionChangesKeys.forComputer(computerId);
  return queryOptions({
    queryKey: keys.bases(workspaceId, sessionId, query),
    queryFn: ({ signal }) =>
      listSessionChangeBases({ workspaceId, sessionId, query, computerId }, signal),
    staleTime: 30_000,
    retry: false,
  });
}
