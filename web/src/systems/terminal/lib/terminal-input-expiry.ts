/**
 * How long an unanswered question has left.
 *
 * The lifetime is a frozen part of the terminal contract rather than a config
 * key: a request lives 15 minutes and then resolves `expired` (`_dx.md`, input
 * requests). Saying how long is left is more useful than saying how long ago it
 * was asked — the reader's decision is whether there is still time to answer.
 */

const REQUEST_LIFETIME_MS = 15 * 60_000;
const MINUTE_MS = 60_000;

export interface TerminalInputExpiry {
  /** Already phrased for the surface. */
  label: string;
  expired: boolean;
}

export function terminalInputExpiry(
  requestedAt: string,
  now: number = Date.now()
): TerminalInputExpiry {
  const asked = Date.parse(requestedAt);
  if (Number.isNaN(asked)) return { label: "", expired: false };
  const remaining = asked + REQUEST_LIFETIME_MS - now;
  if (remaining <= 0) return { label: "expired", expired: true };
  const minutes = Math.max(1, Math.ceil(remaining / MINUTE_MS));
  return { label: `expires in ${minutes}m`, expired: false };
}
