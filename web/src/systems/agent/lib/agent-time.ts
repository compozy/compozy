const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

function span(ms: number) {
  if (ms < HOUR) return `${Math.max(1, Math.round(ms / MINUTE))}m`;
  if (ms < DAY) return `${Math.round(ms / HOUR)}h`;
  return `${Math.round(ms / DAY)}d`;
}

/** "12m ago" or "in 18m"; nothing for a time the daemon hasn't scheduled. */
export function formatRelativeClock(iso: string | null | undefined, now = Date.now()) {
  if (!iso) return null;
  const at = Date.parse(iso);
  if (Number.isNaN(at)) return null;
  const delta = at - now;
  return delta >= 0 ? `in ${span(delta)}` : `${span(-delta)} ago`;
}
