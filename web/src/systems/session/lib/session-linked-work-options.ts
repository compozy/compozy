import { infiniteQueryOptions, keepPreviousData, queryOptions } from "@tanstack/react-query";

import { isSessionUnavailable } from "../adapters/session-api-errors";
import {
  fetchSessionLinkedWork,
  searchSessionLinkCandidates,
  type SessionLinkCandidateKind,
} from "../adapters/session-linked-work-api";
import { sessionKeys } from "./query-keys";
import { PULL_REQUEST_REREAD_INTERVAL_MS } from "./session-pull-request";

const SESSION_LINKED_WORK_STALE_TIME_MS = 2_000;
const SESSION_LINK_CANDIDATES_PAGE = 30;
const SESSION_LINK_CANDIDATES_STALE_TIME_MS = 30_000;

/** The full list; the catalog stream wakes it, and a visible list rereads stale PR facts. */
export function sessionLinkedWorkOptions(workspace: string, id: string) {
  return queryOptions({
    queryKey: sessionKeys.linkedWork(workspace, id),
    queryFn: ({ signal }) => fetchSessionLinkedWork(workspace, id, signal),
    refetchInterval: PULL_REQUEST_REREAD_INTERVAL_MS,
    staleTime: SESSION_LINKED_WORK_STALE_TIME_MS,
    retry: (count, error) => !isSessionUnavailable(error) && count < 1,
    enabled: !!id,
  });
}

/** The server searches and pages; a loaded page is never filtered here. */
export function sessionLinkCandidatesOptions(
  workspace: string,
  id: string,
  kind: SessionLinkCandidateKind,
  query: string
) {
  return infiniteQueryOptions({
    queryKey: sessionKeys.linkCandidates(workspace, id, kind, query),
    queryFn: ({ pageParam, signal }) =>
      searchSessionLinkCandidates(
        workspace,
        id,
        { q: query, kind, page: pageParam, limit: SESSION_LINK_CANDIDATES_PAGE },
        signal
      ),
    initialPageParam: 1,
    getNextPageParam: (lastPage, _pages, lastPageParam) =>
      lastPage.has_more ? lastPageParam + 1 : undefined,
    placeholderData: keepPreviousData,
    staleTime: SESSION_LINK_CANDIDATES_STALE_TIME_MS,
    retry: false,
    enabled: !!id,
  });
}
