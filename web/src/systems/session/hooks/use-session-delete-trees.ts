import { useQueries } from "@tanstack/react-query";

import { sessionDescendantsOptions } from "../lib/query-options";
import type { SessionTrees } from "../lib/session-tree";
import type { SessionPayload } from "../types";

/**
 * Keeps an open delete confirmation's child sessions current from the daemon. `settled` is false
 * while a read runs or failed, so a yes never rests on a count that could understate the tree.
 */
export function useSessionDeleteTrees(sessions: readonly SessionPayload[] | undefined): {
  trees: SessionTrees;
  settled: boolean;
  onRetry?: () => void;
} {
  const reads = useQueries({
    queries: (sessions ?? []).map(session => ({
      ...sessionDescendantsOptions(session),
      refetchOnMount: "always" as const,
      refetchOnWindowFocus: false,
    })),
  });
  const trees = new Map<string, readonly SessionPayload[]>();
  sessions?.forEach((session, index) => {
    const descendants = reads[index]?.data;
    if (descendants) trees.set(session.id, descendants);
  });
  const failed = reads.filter(read => read.isError && !read.isFetching);
  return {
    trees,
    settled: reads.every(read => read.isSuccess && !read.isFetching),
    ...(failed.length > 0 ? { onRetry: () => failed.forEach(read => void read.refetch()) } : {}),
  };
}
