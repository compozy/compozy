/**
 * Time formatters consumed by the `<Time>` primitive. Lives inside `@agh/ui`
 * because the primitive must respect the package boundary (no `web/` imports).
 *
 * `web/src/lib/format-time.ts` re-exports these so application code can consume
 * them without reaching into the UI package internals.
 */

const RELATIVE_THRESHOLDS = [
  { limit: 60_000, divisor: 1_000, unit: "s" }, // < 1 min → seconds
  { limit: 3_600_000, divisor: 60_000, unit: "m" }, // < 1 hour → minutes
  { limit: 86_400_000, divisor: 3_600_000, unit: "h" }, // < 1 day → hours
  { limit: 604_800_000, divisor: 86_400_000, unit: "d" }, // < 1 week → days
] as const;

const ABSOLUTE_FORMATTER = new Intl.DateTimeFormat(undefined, {
  year: "numeric",
  month: "short",
  day: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

const FALLBACK = "—";

function parseIso(iso: string): number | null {
  const parsed = Date.parse(iso);
  return Number.isFinite(parsed) ? parsed : null;
}

/**
 * Renders a humanised relative time string. Returns `"just now"` when the delta
 * is within 30 seconds. Past timestamps render as `"5m ago"`; future timestamps
 * render as `"in 5m"`. Invalid ISO strings render the `—` sentinel so callers
 * never see a thrown exception in render.
 */
export function formatRelativeTime(iso: string, now: number = Date.now()): string {
  const target = parseIso(iso);
  if (target === null) return FALLBACK;

  const deltaMs = now - target;
  const absMs = Math.abs(deltaMs);
  const future = deltaMs < 0;

  if (absMs < 30_000) return "just now";

  for (const { limit, divisor, unit } of RELATIVE_THRESHOLDS) {
    if (absMs < limit) {
      const value = Math.max(1, Math.floor(absMs / divisor));
      return future ? `in ${value}${unit}` : `${value}${unit} ago`;
    }
  }

  const weeks = Math.floor(absMs / 604_800_000);
  if (weeks < 5) return future ? `in ${weeks}w` : `${weeks}w ago`;
  return formatAbsoluteTime(iso);
}

/**
 * Renders an absolute timestamp using the runtime's default locale settings.
 * Invalid ISO strings render the `—` sentinel.
 */
export function formatAbsoluteTime(iso: string): string {
  const target = parseIso(iso);
  if (target === null) return FALLBACK;
  return ABSOLUTE_FORMATTER.format(new Date(target));
}

/**
 * Formats a millisecond duration to its two largest non-zero units, including days.
 * Returns `"0s"` for non-positive or non-finite durations.
 */
export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return "0s";
  const total = Math.floor(ms / 1_000);
  const days = Math.floor(total / 86_400);
  const hours = Math.floor((total % 86_400) / 3_600);
  const minutes = Math.floor((total % 3_600) / 60);
  const seconds = total % 60;
  // Humanized to two units max ("3d 2h", "5m 12s") — long raw chains like
  // "2176h 38m 26s" are banned product vocabulary.
  const units = [
    { value: days, suffix: "d" },
    { value: hours, suffix: "h" },
    { value: minutes, suffix: "m" },
    { value: seconds, suffix: "s" },
  ];
  const formatted = units
    .filter(unit => unit.value > 0)
    .slice(0, 2)
    .map(unit => `${unit.value}${unit.suffix}`);
  return formatted.join(" ") || "0s";
}

/** Sentinel string returned when an ISO input cannot be parsed. */
export const FORMAT_TIME_FALLBACK = FALLBACK;
