import { useQueries, useQueryClient } from "@tanstack/react-query";

import { SessionNotFoundError } from "../adapters/session-api-errors";
import { sessionDetailOptions } from "../lib/query-options";
import {
  subagentContextRequests,
  withSubagentContext,
  type SubagentContextRequest,
} from "../lib/session-subagent-context";
import type { SessionPayload } from "../types";

/** Lineage depth is unbounded (ADR-003); the walk still never loops forever on bad data. */
const MAX_CONTEXT_READS = 64;

/**
 * The page plus the ancestor rows a viewed or matched subagent needs to nest
 * (S9). Ancestors are read through the session detail query, one hop per
 * resolution, so a chain loads parent by parent; cached hops resolve in the
 * same render. Rows appear once their chain is known.
 */
export function useSubagentContextSessions(
  sessions: readonly SessionPayload[],
  revealed: SessionPayload | null | undefined,
  searching: boolean
): readonly SessionPayload[] {
  const queryClient = useQueryClient();
  const loaded = new Map<string, SessionPayload>();
  const deleted = new Set<string>();
  const reads: SubagentContextRequest[] = [];
  for (let hop = 0; hop < MAX_CONTEXT_READS; hop++) {
    const pending = subagentContextRequests({ sessions, loaded, deleted, revealed, searching });
    let resolved = false;
    for (const request of pending) {
      reads.push(request);
      const { queryKey } = sessionDetailOptions(request.workspaceId, request.sessionId);
      const cached = queryClient.getQueryData<SessionPayload>(queryKey);
      if (cached) {
        loaded.set(request.sessionId, cached);
        resolved = true;
      } else if (queryClient.getQueryState(queryKey)?.error instanceof SessionNotFoundError) {
        // The ancestor is gone; its descendants still show, labelled (COPY.md).
        deleted.add(request.sessionId);
        resolved = true;
      }
    }
    if (!resolved) break;
  }
  // Subscribing re-renders when a pending hop lands, which resolves the next one.
  useQueries({
    queries: reads.map(request => sessionDetailOptions(request.workspaceId, request.sessionId)),
  });
  return withSubagentContext({ sessions, loaded, deleted, revealed, searching });
}
