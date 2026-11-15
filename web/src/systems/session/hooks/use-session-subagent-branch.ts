import { useInfiniteQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import type { SubagentView } from "../components/subagents/types";
import { subagentViewFromPayload } from "../lib/subagent-payload";
import { isActiveSubagent, sortSubagentsByUrgency } from "../lib/subagent-roster";
import {
  SUBAGENT_BRANCH_PAGE_SIZE,
  sessionSubagentBranchOptions,
} from "../lib/subagent-query-options";
import { useLiveSubagentRoster } from "./use-session-subagents";

export interface SessionSubagentBranchModel {
  /**
   * The rows the branch mounts, active work first (waiting, working), then
   * failed, queued and settled, newest first within each: every active row,
   * and at least the pages asked for.
   */
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
 * Both sources order the same way before the branch cuts its rows, and active
 * work never hides behind "Show more": the branch shows every active row past
 * its page size, and reads the next page while the loaded rows are all active
 * (the daemon pages active work first, so more of it may follow). Mount it
 * only while the branch is open: a folded row reads nothing.
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
  const { hasNextPage, isFetching, isFetchingNextPage, fetchNextPage, isError } = query;
  const loaded = sortSubagentsByUrgency(
    live?.rows ??
      query.data?.pages.flatMap(page => page.subagents.map(subagentViewFromPayload)) ??
      []
  );
  const active = loaded.filter(isActiveSubagent).length;
  const complete = live !== null || (query.isSuccess && !hasNextPage);
  const shown = Math.max(limit, active);
  const rows = loaded.slice(0, shown);
  const known = complete ? loaded.length : Math.max(total, loaded.length);
  const failed = live === null && isError;
  // Every loaded row is active, so the next page may hold more active work.
  const activeTail = live === null && loaded.length > 0 && active === loaded.length;
  // Wait for any read in flight: fetchNextPage would cancel a first-page refetch.
  useEffect(() => {
    if (activeTail && hasNextPage && !isFetching && !isError)
      void fetchNextPage().catch(() => undefined);
  }, [activeTail, hasNextPage, isFetching, isError, fetchNextPage]);

  return {
    rows,
    remaining: Math.max(0, known - rows.length),
    loading: live === null && query.isLoading && !failed,
    loadingMore: isFetchingNextPage,
    failed,
    showMore: () => {
      const next = shown + SUBAGENT_BRANCH_PAGE_SIZE;
      setLimit(next);
      if (live === null && loaded.length < next && hasNextPage && !isFetchingNextPage)
        void fetchNextPage().catch(() => undefined);
    },
    retry: () => void query.refetch(),
  };
}
