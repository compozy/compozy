import { useInfiniteQuery } from "@tanstack/react-query";

import { sessionsListOptions } from "../lib/query-options";
import { flattenSessionPages } from "../lib/session-list-query";

export function useGoalVerificationSessions(sessionId: string) {
  const query = useInfiniteQuery(
    sessionsListOptions({ parent: sessionId, type: "system", all_profiles: true })
  );
  return {
    sessions: flattenSessionPages(query.data) ?? [],
    isPending: query.isPending,
    isError: query.isError,
    isSuccess: query.isSuccess,
    hasNextPage: query.hasNextPage,
    isFetchingNextPage: query.isFetchingNextPage,
    refetch: query.refetch,
    fetchNextPage: query.fetchNextPage,
  };
}
