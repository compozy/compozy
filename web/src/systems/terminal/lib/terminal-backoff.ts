/**
 * How long to wait before trying the connection again.
 *
 * Exponential, with jitter. The window-manager precedent has none; spreading
 * the retries is an intentional delta here, because every terminal pane in a
 * workspace would otherwise wake at the same millisecond after a daemon
 * restart and arrive as one thundering reconnect.
 */

const MAX_BACKOFF_MS = 8_000;
const BASE_BACKOFF_MS = 500;
const MAX_BACKOFF_EXPONENT = 4;

/** Cancels a scheduled run. */
export type CancelScheduled = () => void;
export type ScheduleFn = (run: () => void, delayMs: number) => CancelScheduled;

/** Half the ceiling, plus up to half again: never zero, never synchronized. */
export function terminalBackoffDelay(attempt: number, random: () => number = Math.random): number {
  const exponent = Math.min(attempt, MAX_BACKOFF_EXPONENT);
  const ceiling = Math.min(MAX_BACKOFF_MS, BASE_BACKOFF_MS * 2 ** exponent);
  return ceiling / 2 + random() * (ceiling / 2);
}

export const defaultSchedule: ScheduleFn = (run, delayMs) => {
  const timer = setTimeout(run, delayMs);
  return () => clearTimeout(timer);
};
