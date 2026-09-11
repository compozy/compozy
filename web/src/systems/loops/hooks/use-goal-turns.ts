import { useInfiniteQuery } from "@tanstack/react-query";
import { useProfileReadScope } from "@/systems/profiles";
import { goalTurnsOptions } from "../lib/query-options";
import type { GoalTurn } from "../types";

export interface GoalTurnsRead {
  turns: readonly GoalTurn[];
  isLoading: boolean;
  isError: boolean;
  hasMore: boolean;
  isLoadingMore: boolean;
  onLoadMore?: () => void;
  onRetry?: () => void;
}

/** Projects profile-scoped Goal pages into ordered history and continuation controls. */
export function useGoalTurns(
  workspaceId: string,
  runId: string,
  enabled: boolean,
  isLive: boolean
): GoalTurnsRead {
  const { key } = useProfileReadScope();
  const query = useInfiniteQuery(goalTurnsOptions(workspaceId, runId, enabled, isLive, key));
  return {
    turns: query.data?.pages.flatMap(page => page.turns) ?? [],
    isLoading: query.isLoading,
    isError: query.isError,
    hasMore: query.hasNextPage,
    isLoadingMore: query.isFetchingNextPage,
    /** Loads the next server cursor into the existing ordered history. */
    onLoadMore: () => {
      void query.fetchNextPage();
    },
    /** Retries the current history read after a recoverable failure. */
    onRetry: () => {
      void query.refetch();
    },
  };
}
