// Duration words for a supervision quiet episode: the live "quiet for 12m"
// segment on the working row and the frozen "no work for 40 minutes" of a
// legacy inactivity stop. The episode itself is the daemon's
// `supervision.quiet_warning`, present only while a turn is in flight.

/**
 * Minute-granular status vocabulary ("31m", "1h 5m"); seconds only under a
 * minute so short configured thresholds still read truthfully.
 */
export function formatQuietDuration(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1_000));
  if (totalSeconds < 60) {
    return `${totalSeconds}s`;
  }
  const hours = Math.floor(totalSeconds / 3_600);
  const minutes = Math.floor((totalSeconds % 3_600) / 60);
  if (hours > 0) {
    return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`;
  }
  return `${minutes}m`;
}

function pluralize(value: number, unit: string): string {
  return `${value} ${unit}${value === 1 ? "" : "s"}`;
}

/** Spelled-out duration for a sentence ("30 minutes", "1 hour 10 minutes", "45 seconds"). */
export function formatQuietDurationWords(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1_000));
  if (totalSeconds < 60) {
    return pluralize(totalSeconds, "second");
  }
  const hours = Math.floor(totalSeconds / 3_600);
  const minutes = Math.floor((totalSeconds % 3_600) / 60);
  if (hours > 0) {
    return minutes > 0
      ? `${pluralize(hours, "hour")} ${pluralize(minutes, "minute")}`
      : pluralize(hours, "hour");
  }
  return pluralize(minutes, "minute");
}
