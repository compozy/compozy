import { useQueries } from "@tanstack/react-query";

import { sessionDescendantsOptions, type SessionTrees } from "../lib/session-tree";
import type { SessionPayload } from "../types";

/**
 * Keeps an open delete confirmation's child sessions current from the daemon. Only a settled read
 * counts: a cached tree mid-refetch or after a failure could understate what a yes deletes.
 */
export function useSessionDeleteTrees(
  sessions: readonly SessionPayload[] | undefined
): SessionTrees {
  const reads = useQueries({
    queries: (sessions ?? []).map(session => ({
      ...sessionDescendantsOptions(session),
      refetchOnMount: "always" as const,
      refetchOnWindowFocus: false,
    })),
  });
  const trees = new Map<string, readonly SessionPayload[]>();
  sessions?.forEach((session, index) => {
    const read = reads[index];
    if (read?.isSuccess && !read.isFetching) trees.set(session.id, read.data);
  });
  return trees;
}
