import { useInfiniteQuery } from "@tanstack/react-query";
import { useState } from "react";

import type { SubagentView } from "../components/subagents/types";
import { subagentViewFromPayload } from "../lib/subagent-payload";
import {
  SUBAGENT_BRANCH_PAGE_SIZE,
  sessionSubagentBranchOptions,
} from "../lib/subagent-query-options";
import { useLiveSubagentRoster } from "./use-session-subagents";

export interface SessionSubagentBranchModel {
  /** The rows the branch mounts, newest first: at most the pages asked for. */
  rows: readonly SubagentView[];
  /** Subagents past the shown rows, loaded or not. */
  remaining: number;
  /** The first page is still in flight. */
  loading: boolean;
  /** "Show more" is reading the next page. */
  loadingMore: boolean;
  /** The list read failed and no stream roster stands in. */
  failed: boolean;
  showMore: () => void;
  retry: () => void;
}

/**
 * A session row's open subagent branch. While the parent's session stream
 * holds a live roster it feeds the rows and no request runs; otherwise the
 * list route serves short pages, the next one only when "Show more" needs it.
 * Mount it only while the branch is open: a folded row reads nothing.
 */
export function useSessionSubagentBranch(
  workspaceId: string,
  sessionId: string,
  /** The row's `subagent_summary.total`, the count before any page loads. */
  total: number
): SessionSubagentBranchModel {
  const [limit, setLimit] = useState(SUBAGENT_BRANCH_PAGE_SIZE);
  const scoped = workspaceId !== "" && sessionId !== "";
  const live = useLiveSubagentRoster(workspaceId, sessionId, scoped);
  const query = useInfiniteQuery({
    ...sessionSubagentBranchOptions(workspaceId, sessionId),
    enabled: scoped && live === null,
  });
  const loaded =
    live?.rows ??
    query.data?.pages.flatMap(page => page.subagents.map(subagentViewFromPayload)) ??
    [];
  const complete = live !== null || (query.isSuccess && !query.hasNextPage);
  const rows = loaded.slice(0, limit);
  const known = complete ? loaded.length : Math.max(total, loaded.length);
  const failed = live === null && query.isError;

  return {
    rows,
    remaining: Math.max(0, known - rows.length),
    loading: live === null && query.isLoading && !failed,
    loadingMore: query.isFetchingNextPage,
    failed,
    showMore: () => {
      const next = limit + SUBAGENT_BRANCH_PAGE_SIZE;
      setLimit(next);
      if (live === null && loaded.length < next && query.hasNextPage && !query.isFetchingNextPage)
        void query.fetchNextPage().catch(() => undefined);
    },
    retry: () => void query.refetch(),
  };
}
