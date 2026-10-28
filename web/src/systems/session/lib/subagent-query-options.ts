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

/** One session-row branch page: the most rows a branch mounts before "Show more". */
export const SUBAGENT_BRANCH_PAGE_SIZE = 8;

/**
 * Short pages of one parent's direct subagents for its session-row branch,
 * most urgent first: the daemon orders before it cuts the page, so a waiting
 * or failed row never hides behind "Show more". A branch reads only while open
 * and pages on demand, so a long roster never loads (or mounts) more than the
 * operator asked to see.
 */
export function sessionSubagentBranchOptions(workspaceId: string, sessionId: string) {
  return infiniteQueryOptions({
    queryKey: sessionKeys.subagentBranch(workspaceId, sessionId),
    queryFn: ({ pageParam, signal }) =>
      fetchSessionSubagents(
        workspaceId,
        sessionId,
        {
          limit: SUBAGENT_BRANCH_PAGE_SIZE,
          order: "urgency",
          ...(pageParam ? { cursor: pageParam } : {}),
        },
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
