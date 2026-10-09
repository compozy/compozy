import type { SessionPayload } from "../types";
import { sessionBadgeSignal } from "./session-badge";
import { isSessionRunning } from "./session-running";

/** Chip counts, structurally the subagent components' `SubagentCounts`. */
export interface SessionSubagentCounts {
  live: number;
  total: number;
  failed: number;
  attention: number;
}

/** The parent row's `subagent_summary` as chip counts; `null` when it has none. */
export function sessionSubagentCounts(session: SessionPayload): SessionSubagentCounts | null {
  const summary = session.subagent_summary;
  if (!summary || summary.total <= 0) return null;
  return {
    live: summary.live,
    total: summary.total,
    failed: summary.failed,
    attention: summary.attention,
  };
}

/**
 * A parent with no running turn whose subagents still work reads `delegated`,
 * not done or idle (UT-W14). Needs-you, failure and stop marks keep priority.
 */
export function sessionAwaitsSubagents(session: SessionPayload): boolean {
  const live = session.subagent_summary?.live ?? 0;
  if (live <= 0 || isSessionRunning(session)) return false;
  const { state } = sessionBadgeSignal(session.badge);
  return state === "done" || state === "idle";
}
