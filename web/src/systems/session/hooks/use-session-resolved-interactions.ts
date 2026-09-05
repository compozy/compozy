import { useQuery } from "@tanstack/react-query";

import { sessionResolvedInteractionsOptions } from "../lib/query-options";
import { resolvedInteractionsByRequest } from "../lib/session-pending-interactions";
import type { SessionInteractionRecord } from "../types";

const NO_RESOLVED_INTERACTIONS: ReadonlyMap<string, SessionInteractionRecord> = new Map();

/**
 * Decisions the daemon applied, keyed by provider request id, for receipt attribution.
 * `decidedRequestIds` are the receipts the transcript shows; the read runs while there
 * is at least one and re-reads once per newly decided ask. An unreadable projection
 * degrades to "no attribution" — the receipt stays neutral, never a guessed actor.
 */
export function useSessionResolvedInteractions(
  workspaceId: string,
  sessionId: string,
  options: { enabled?: boolean; decidedRequestIds?: ReadonlySet<string> } = {}
): ReadonlyMap<string, SessionInteractionRecord> {
  const query = useQuery(sessionResolvedInteractionsOptions(workspaceId, sessionId, options));
  return query.data ? resolvedInteractionsByRequest(query.data) : NO_RESOLVED_INTERACTIONS;
}
