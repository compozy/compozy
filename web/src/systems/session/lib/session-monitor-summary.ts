import type { SessionPayload } from "../types";

/** Monitors still waiting to wake the session (active or delivering). */
export function sessionActiveMonitors(session: Pick<SessionPayload, "monitor_summary">): number {
  return session.monitor_summary?.active ?? 0;
}
