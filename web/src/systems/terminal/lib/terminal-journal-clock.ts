export interface TerminalJournalClockOptions {
  /** The rail needs seconds; the row scans at minute resolution. */
  seconds?: boolean;
}

function pad2(value: number): string {
  return String(value).padStart(2, "0");
}

/**
 * Operational wall-clock for journal rows and the record rail.
 *
 * Fixed 24-hour from the instant's UTC fields, because these are values
 * compared down a column — an AM/PM suffix or a locale-dependent pad would
 * break the tabular alignment the scan depends on.
 */
export function formatTerminalJournalClock(
  iso: string,
  options: TerminalJournalClockOptions = {}
): string {
  const parsed = new Date(iso);
  if (Number.isNaN(parsed.getTime())) return iso;
  const hours = pad2(parsed.getUTCHours());
  const minutes = pad2(parsed.getUTCMinutes());
  if (!options.seconds) return `${hours}:${minutes}`;
  return `${hours}:${minutes}:${pad2(parsed.getUTCSeconds())}`;
}
