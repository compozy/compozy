import { LOCAL_COMPUTER_ID, queryKeyComputerId, type ComputerId } from "@/lib/computer-scope";
import { queryOptions } from "@tanstack/react-query";

import { fetchSessionSides } from "../adapters/session-side-api";
import { sessionKeys } from "./query-keys";

/**
 * A session's side chats. The list is complete and small (bounded by the operator's own actions),
 * so it is one read; the catalog stream's `session_side_changed` for this parent rereads it.
 */
export function sessionSidesOptions(
  workspaceId: string,
  sessionId: string,
  options: { archived?: boolean; enabled?: boolean; computerId?: ComputerId } = {}
) {
  const archived = options.archived === true;
  return queryOptions({
    queryKey: sessionKeys
      .forComputer(options.computerId)
      .sidesList(workspaceId, sessionId, archived),
    queryFn: ({ signal }) =>
      fetchSessionSides(workspaceId, sessionId, { archived }, signal, options.computerId),
    select: data => data.sides,
    staleTime: 30_000,
    enabled: (options.enabled ?? true) && sessionId.trim() !== "",
  });
}

/** Matches every side list of `parentSessionId`, whatever workspace selector the window used. */
export function isSessionSidesQuery(
  queryKey: readonly unknown[],
  parentSessionId?: string,
  computerId: ComputerId = LOCAL_COMPUTER_ID
): boolean {
  if (queryKeyComputerId(queryKey) !== computerId) return false;
  const sides = queryKey.indexOf("sides");
  if (sides < 2 || queryKey[sides - 2] !== "detail") return false;
  return parentSessionId === undefined || queryKey[sides - 1] === parentSessionId;
}
