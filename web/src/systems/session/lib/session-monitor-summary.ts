import type { SessionPayload } from "../types";
import { sessionBadgeSignal } from "./session-badge";
import { isSessionRunning } from "./session-running";

/** Monitors still waiting to wake the session (active or delivering). */
export function sessionActiveMonitors(session: Pick<SessionPayload, "monitor_summary">): number {
  return session.monitor_summary?.active ?? 0;
}

/**
 * A session with no running turn whose monitors still wait to wake it reads
 * `monitoring`, not done or idle. Needs-you, failure and stop marks keep priority.
 */
export function sessionAwaitsMonitors(session: SessionPayload): boolean {
  if (sessionActiveMonitors(session) <= 0 || isSessionRunning(session)) return false;
  const { state } = sessionBadgeSignal(session.badge);
  return state === "done" || state === "idle";
}
