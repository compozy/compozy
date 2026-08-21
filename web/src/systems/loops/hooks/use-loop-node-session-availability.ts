import { useQuery } from "@tanstack/react-query";

import { SessionNotFoundError, sessionDetailOptions } from "@/systems/session";

/**
 * Whether the session a step recorded is still there to open.
 *
 * The roster carries `session_id` but no availability: the id is a durable
 * relational fact, and whether the session survived retention is the session
 * store's business, not the run's. So the truth comes from the session read
 * itself — a 404 is exactly the daemon saying "pruned".
 *
 * This asks for **one** session, the one whose node is open in the panel. Asking
 * per roster row would be an N+1 walk of the whole run to answer a question
 * nobody asked, which is the pattern the runs roster just finished deleting.
 */
export type LoopNodeSessionAvailability = "available" | "pruned" | "unknown";

export function useLoopNodeSessionAvailability(
  workspaceId: string,
  sessionId: string | null,
  enabled = true
): LoopNodeSessionAvailability {
  const options = sessionDetailOptions(workspaceId, sessionId ?? "");
  const query = useQuery({
    ...options,
    enabled: Boolean(workspaceId) && Boolean(sessionId) && enabled,
    // A pruned session is a settled answer, not a transport failure: retrying it
    // three times only delays the sentence the panel is going to print anyway.
    retry: (failureCount, error) => !(error instanceof SessionNotFoundError) && failureCount < 2,
  });
  if (query.error instanceof SessionNotFoundError) return "pruned";
  // Anything else — still loading, offline, a 500 — is not evidence of pruning,
  // and guessing "gone" from a transport blip would delete a working link.
  return query.data ? "available" : "unknown";
}

/**
 * The pruned-session set the node panel reads, holding at most the one session
 * the open node recorded. Empty means "nothing known to be gone", never "gone".
 */
export function loopPrunedSessionIds(
  sessionId: string | null,
  availability: LoopNodeSessionAvailability
): ReadonlySet<string> | undefined {
  if (!sessionId || availability !== "pruned") return undefined;
  return new Set([sessionId]);
}
