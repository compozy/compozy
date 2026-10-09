import { infiniteQueryOptions } from "@tanstack/react-query";

import { fetchSessionSubagents } from "../adapters/subagent-api";
import { sessionKeys } from "./query-keys";

/** The list route's maximum page; the roster needs every direct child to pin failures. */
export const SUBAGENT_LIST_PAGE_SIZE = 200;

/**
 * Every page of one parent's direct subagents, newest first. Catalog upserts
 * for the parent invalidate it, so the roster and chip preview stay live.
 */
export function sessionSubagentsOptions(workspaceId: string, sessionId: string) {
  return infiniteQueryOptions({
    queryKey: sessionKeys.subagents(workspaceId, sessionId),
    queryFn: ({ pageParam, signal }) =>
      fetchSessionSubagents(
        workspaceId,
        sessionId,
        { limit: SUBAGENT_LIST_PAGE_SIZE, ...(pageParam ? { cursor: pageParam } : {}) },
        signal
      ),
    initialPageParam: "",
    getNextPageParam: page => page.next_cursor ?? undefined,
    enabled: workspaceId !== "" && sessionId !== "",
    staleTime: 5_000,
    refetchOnWindowFocus: false,
    retry: false,
  });
}
