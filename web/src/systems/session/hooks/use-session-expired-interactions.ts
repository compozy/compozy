import { useQuery } from "@tanstack/react-query";

import { sessionExpiredInteractionsOptions } from "../lib/query-options";
import { expiredInteractionsByRequest } from "../lib/session-pending-interactions";
import type { SessionInteractionRecord } from "../types";
import { useSessionCatalogStreamLive } from "./use-session-catalog-streams";

const NO_EXPIRED_INTERACTIONS: ReadonlyMap<string, SessionInteractionRecord> = new Map();

/**
 * Decisions the daemon settled without a transcript answer, keyed by provider request id.
 * `undecidedRequestIds` are the asks the transcript still shows open: the read runs while
 * there is at least one, and keeps re-reading only while one of them is still unknown to
 * the settled map. An unreadable projection degrades to "nothing settled", never to an
 * invented outcome; a cached map stays visible after polling stops.
 */
export function useSessionExpiredInteractions(
  workspaceId: string,
  sessionId: string,
  options: {
    enabled?: boolean;
    undecidedRequestIds?: ReadonlySet<string>;
    /** The session's owner profile; the catalog stream covers only its own scope. */
    profileName?: string;
  } = {}
): ReadonlyMap<string, SessionInteractionRecord> {
  const { profileName, ...readOptions } = options;
  const streamCovered = useSessionCatalogStreamLive(profileName);
  const query = useQuery(
    sessionExpiredInteractionsOptions(workspaceId, sessionId, { ...readOptions, streamCovered })
  );
  return query.data ? expiredInteractionsByRequest(query.data) : NO_EXPIRED_INTERACTIONS;
}
