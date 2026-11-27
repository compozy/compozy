import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";
import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import { fetchSessionSubagents, type SubagentPayload } from "../adapters/subagent-api";
import { REMOTE_SESSION_REFETCH_MS } from "./session-catalog-options";
import { sessionKeys } from "./query-keys";
import { EMPTY_SUBAGENT_ROSTER } from "./subagent-roster";

/** The list route's maximum page; the roster needs every direct child to pin failures. */
export const SUBAGENT_LIST_PAGE_SIZE = 200;

/**
 * Every page of one parent's direct subagents, newest first. Catalog upserts
 * for the parent invalidate it, so the roster and chip preview stay live.
 */
export function sessionSubagentsOptions(
  workspaceId: string,
  sessionId: string,
  computerId?: ComputerId
) {
  return infiniteQueryOptions({
    queryKey: sessionKeys.forComputer(computerId).subagents(workspaceId, sessionId),
    queryFn: ({ pageParam, signal }) =>
      fetchSessionSubagents(
        workspaceId,
        sessionId,
        { limit: SUBAGENT_LIST_PAGE_SIZE, ...(pageParam ? { cursor: pageParam } : {}) },
        signal,
        ...(isRemoteComputerId(computerId) && computerId ? ([computerId] as const) : [])
      ),
    initialPageParam: "",
    getNextPageParam: page => page.next_cursor ?? undefined,
    enabled: workspaceId !== "" && sessionId !== "",
    staleTime: 5_000,
    ...(isRemoteComputerId(computerId) ? { refetchInterval: REMOTE_SESSION_REFETCH_MS } : {}),
    refetchOnWindowFocus: false,
    retry: false,
  });
}

/** One session-row branch page: the most rows a branch mounts before "Show more". */
export const SUBAGENT_BRANCH_PAGE_SIZE = 8;

/**
 * Short pages of one parent's direct subagents for its session-row branch,
 * active work first (waiting, working, then failed, queued, settled): the
 * daemon orders before it cuts the page, so active work never hides behind a
 * later page. A branch reads only while open
 * and pages on demand, so a long roster never loads (or mounts) more than the
 * operator asked to see.
 */
export function sessionSubagentBranchOptions(
  workspaceId: string,
  sessionId: string,
  computerId?: ComputerId
) {
  return infiniteQueryOptions({
    queryKey: sessionKeys.forComputer(computerId).subagentBranch(workspaceId, sessionId),
    queryFn: ({ pageParam, signal }) =>
      fetchSessionSubagents(
        workspaceId,
        sessionId,
        {
          limit: SUBAGENT_BRANCH_PAGE_SIZE,
          order: "urgency",
          ...(pageParam ? { cursor: pageParam } : {}),
        },
        signal,
        ...(computerId === undefined ? [] : ([computerId] as const))
      ),
    initialPageParam: "",
    getNextPageParam: page => page.next_cursor ?? undefined,
    enabled: workspaceId !== "" && sessionId !== "",
    staleTime: 5_000,
    ...(isRemoteComputerId(computerId) ? { refetchInterval: REMOTE_SESSION_REFETCH_MS } : {}),
    refetchOnWindowFocus: false,
    retry: false,
  });
}

/**
 * The parent's subagent roster is stream-owned: the session stream's snapshot
 * and row updates write it, and nothing fetches it.
 */
export function sessionSubagentRosterOptions(
  workspaceId: string,
  sessionId: string,
  computerId?: ComputerId
) {
  return queryOptions({
    queryKey: sessionKeys.forComputer(computerId).subagentRoster(workspaceId, sessionId),
    queryFn: () => EMPTY_SUBAGENT_ROSTER,
    enabled: false,
  });
}

/**
 * Every subagent of the parent for the mid-prompt control poll (newest first). The
 * poll replaces the whole roster, so it follows the cursor: a row left out would
 * read as an unconfirmed, still-running card.
 */
export function sessionSubagentRosterPollOptions(
  workspaceId: string,
  sessionId: string,
  computerId?: ComputerId
) {
  return queryOptions({
    queryKey: sessionKeys.forComputer(computerId).subagentRosterPoll(workspaceId, sessionId),
    queryFn: async ({ signal }) => {
      const subagents: SubagentPayload[] = [];
      let cursor: string | undefined;
      do {
        const page = await fetchSessionSubagents(
          workspaceId,
          sessionId,
          cursor ? { limit: 200, cursor } : { limit: 200 },
          signal,
          ...(isRemoteComputerId(computerId) && computerId ? ([computerId] as const) : [])
        );
        subagents.push(...page.subagents);
        cursor = page.next_cursor ?? undefined;
      } while (cursor);
      return { subagents, next_cursor: null };
    },
    staleTime: 0,
    refetchOnWindowFocus: false,
    retry: false,
  });
}
