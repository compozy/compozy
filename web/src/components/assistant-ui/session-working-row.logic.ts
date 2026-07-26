// Live "Working for Xs" formatting: floor to whole seconds from the turn start
// (0s on the first tick), roll up to `Xm Ys` past a minute and `Xh Ym` past an
// hour. Distinct from the settled fold's `formatDuration` (which rounds to a
// minimum of 1s) because the live timer counts up from 0 while the turn is
// still in flight.
export function formatWorkingElapsed(startedAtMs: number, nowMs: number): string {
  const totalSeconds = Math.max(0, Math.floor((nowMs - startedAtMs) / 1000));
  if (totalSeconds < 60) {
    return `${totalSeconds}s`;
  }
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) {
    return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`;
  }
  return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`;
}
