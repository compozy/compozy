import type { SessionPayload } from "../types";

/**
 * The daemon's attention code for a stop whose ladder ran out without a
 * verified death (`internal/session/stop_verification.go`). It is durable on
 * the session payload until the daemon itself reads `stopped`.
 */
export const STOP_VERIFICATION_FAILED_ATTENTION = "stop_verification_failed";

export type SessionStopAttention = typeof STOP_VERIFICATION_FAILED_ATTENTION;

/**
 * The stop attention the page must surface, read from the payload the daemon
 * projects on GET, list, and catalog snapshots. Only a non-terminal session
 * carries it: once the daemon reads `stopped` the attention is history, and a
 * fabricated one here would contradict the badge.
 */
export function sessionStopAttention(
  session: Pick<SessionPayload, "attention" | "state">
): SessionStopAttention | null {
  return session.attention === STOP_VERIFICATION_FAILED_ATTENTION && session.state !== "stopped"
    ? STOP_VERIFICATION_FAILED_ATTENTION
    : null;
}
