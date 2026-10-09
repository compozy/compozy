import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { cancelSubagent } from "../adapters/subagent-api";
import type { SubagentView } from "../components/subagents/types";
import { sessionKeys } from "../lib/query-keys";
import { subagentViewFromPayload } from "../lib/subagent-payload";
import { sessionSubagentsOptions } from "../lib/subagent-query-options";

export interface SessionSubagentsModel {
  subagents: SubagentView[];
  loading: boolean;
  failed: boolean;
  /** Cancels one delegated subagent; rejects so the row raises its stop-failed toast. */
  stop: (subagent: SubagentView) => Promise<unknown>;
}

/**
 * Direct subagents of one parent session from the paged list route. The roster
 * orders failed → live → previous over the whole set, so every page is read;
 * the population is the parent's direct children, never the workspace.
 */
export function useSessionSubagents(
  workspaceId: string,
  sessionId: string,
  enabled = true
): SessionSubagentsModel {
  const queryClient = useQueryClient();
  const query = useInfiniteQuery({
    ...sessionSubagentsOptions(workspaceId, sessionId),
    enabled: enabled && workspaceId !== "" && sessionId !== "",
  });
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = query;
  useEffect(() => {
    if (hasNextPage && !isFetchingNextPage) void fetchNextPage().catch(() => undefined);
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  return {
    subagents: query.data?.pages.flatMap(page => page.subagents.map(subagentViewFromPayload)) ?? [],
    loading: query.isLoading,
    failed: query.isError,
    stop: async subagent => {
      await cancelSubagent(workspaceId, subagent.id);
      await queryClient.invalidateQueries({
        queryKey: sessionKeys.subagents(workspaceId, sessionId),
      });
    },
  };
}
