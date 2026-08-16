import {
  type SessionCatalogStreamStatus,
  type SessionPayload,
  useSessions,
} from "@/systems/session";

const ATTENTION_ROW_LIMIT = 100;

export interface OsAttentionSessionsModel {
  sessions: SessionPayload[];
  stale: boolean;
  loading: boolean;
}

/**
 * Cross-workspace attention rows for the bell: needs-you and finished-unseen
 * sessions from every workspace the operator owns. Two server-filtered queries
 * rather than one client-side filter — the daemon owns the needs-you class, and
 * a filtered read keeps the page bounded around what the bell actually shows.
 *
 * Rows survive a stale stream on purpose (US-005.EC-2): the operator can still
 * jump from a frozen row. Staleness travels with them so they contribute
 * nothing to any count and fire no notification.
 */
export function useAttentionSessions(
  streamStatus: SessionCatalogStreamStatus,
  enabled: boolean
): OsAttentionSessionsModel {
  const needsYou = useSessions(null, {
    enabled,
    loadAll: true,
    filters: {
      attention: true,
      include_health: true,
      limit: ATTENTION_ROW_LIMIT,
      sort: "attention",
    },
  });
  const finished = useSessions(null, {
    enabled,
    loadAll: true,
    filters: { badge: "done", include_health: true, limit: ATTENTION_ROW_LIMIT, sort: "attention" },
  });
  return {
    sessions: [...(needsYou.data ?? []), ...(finished.data ?? [])],
    stale:
      !enabled ||
      streamStatus !== "live" ||
      needsYou.isError ||
      finished.isError ||
      needsYou.data === undefined ||
      finished.data === undefined,
    loading: enabled && (needsYou.isLoading || finished.isLoading),
  };
}
