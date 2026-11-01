import { useQueries } from "@tanstack/react-query";

import { sessionDescendantsOptions } from "../lib/query-options";
import type { SessionTrees } from "../lib/session-tree";
import type { SessionPayload } from "../types";

/** Keeps an open delete confirmation's child sessions current from the daemon. */
export function useSessionDeleteTrees(
  sessions: readonly SessionPayload[] | undefined,
): SessionTrees {
  const reads = useQueries({
    queries: (sessions ?? []).map((session) => ({
      ...sessionDescendantsOptions(session),
      refetchOnMount: "always" as const,
    })),
  });
  const trees = new Map<string, readonly SessionPayload[]>();
  sessions?.forEach((session, index) => {
    const descendants = reads[index]?.data;
    if (descendants) trees.set(session.id, descendants);
  });
  return trees;
}
