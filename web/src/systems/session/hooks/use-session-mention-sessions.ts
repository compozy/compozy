import { useDebouncedValue } from "@/hooks/use-debounced-input";

import type { SessionPayload } from "../types";
import { useSessionCatalog } from "./use-session-catalog";
import {
  SESSION_MENTION_DEBOUNCE_MS,
  type SessionMentionSourceStatus,
} from "./use-session-file-search";

/** The picker's "Sessions" group shows at most five rows (Business Rule 13). */
export const SESSION_MENTION_SESSION_LIMIT = 5;

export interface SessionMentionSessionsView {
  sessions: readonly SessionPayload[];
  status: SessionMentionSourceStatus;
}

/**
 * Same-workspace sessions for the `@` picker through the existing catalog
 * (`listSessions` with `q`, newest first, archived excluded by the daemon).
 * The catalog has no "exclude this id" filter, so the page asks for one extra
 * row and drops the session being prompted before the five-row cut.
 */
export function useSessionMentionSessions({
  workspaceId,
  sessionId,
  query,
  enabled,
}: {
  workspaceId: string;
  sessionId: string;
  query: string;
  enabled: boolean;
}): SessionMentionSessionsView {
  const debounced = useDebouncedValue(query.trim(), SESSION_MENTION_DEBOUNCE_MS);
  const catalog = useSessionCatalog(
    workspaceId,
    { q: debounced, limit: SESSION_MENTION_SESSION_LIMIT + 1 },
    enabled && workspaceId !== ""
  );
  const sessions = catalog.sessions
    .filter(session => session.id !== sessionId)
    .slice(0, SESSION_MENTION_SESSION_LIMIT);
  const status: SessionMentionSourceStatus = catalog.failed
    ? "error"
    : catalog.loading
      ? "loading"
      : "ready";
  return { sessions, status };
}
