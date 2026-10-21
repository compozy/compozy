import type { ProviderUsageAmount, ProviderUsageWindow } from "../types";

const MINUTE_MS = 60_000;
const HOUR_MINUTES = 60;
const DAY_MINUTES = 1_440;
const ABSOLUTE_RESET_AFTER_MS = 7 * 24 * 60 * MINUTE_MS;

const RESET_DATE_FORMAT = new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric" });

function parseTime(iso: string | null | undefined): number | null {
  if (!iso) return null;
  const parsed = Date.parse(iso);
  return Number.isFinite(parsed) ? parsed : null;
}

/** Whole percent used; below 100 never rounds up into a value that reads exhausted. */
export function formatPercent(usedPercent: number): string {
  const rounded = Math.round(usedPercent);
  return `${usedPercent < 100 ? Math.min(rounded, 99) : rounded}%`;
}

/** Bar fill: the provider may report above 100, the bar stops at full. */
export function clampPercent(usedPercent: number): number {
  return Math.min(100, Math.max(0, usedPercent));
}

function formatDurationMinutes(totalMinutes: number): string {
  if (totalMinutes >= DAY_MINUTES) {
    const days = Math.floor(totalMinutes / DAY_MINUTES);
    const hours = Math.floor((totalMinutes % DAY_MINUTES) / HOUR_MINUTES);
    return hours > 0 ? `${days}d ${hours}h` : `${days}d`;
  }
  if (totalMinutes >= HOUR_MINUTES) {
    const hours = Math.floor(totalMinutes / HOUR_MINUTES);
    const minutes = totalMinutes % HOUR_MINUTES;
    return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`;
  }
  return totalMinutes < 1 ? "<1m" : `${totalMinutes}m`;
}

/**
 * Reset fragment for an absolute reset time, derived from the client clock:
 * "resets in 2d 4h", "Resetting…" once the time has passed (clock skew never
 * yields a negative value), or the date ("Resets Oct 28") beyond a week.
 */
export function formatResetsAt(resetsAt: string | null, now: number): string | null {
  const target = parseTime(resetsAt);
  if (target === null) return null;
  const delta = target - now;
  if (delta <= 0) return "Resetting…";
  if (delta > ABSOLUTE_RESET_AFTER_MS) return `Resets ${RESET_DATE_FORMAT.format(target)}`;
  return `resets in ${formatDurationMinutes(Math.floor(delta / MINUTE_MS))}`;
}

/**
 * A window's reset fragment: the live countdown when the provider gave a time,
 * otherwise its reset text verbatim, otherwise nothing (no invented countdown).
 */
export function formatWindowReset(
  window: Pick<ProviderUsageWindow, "resets_at" | "reset_text">,
  now: number
): string | null {
  const countdown = formatResetsAt(window.resets_at, now);
  if (countdown !== null) return countdown;
  const text = window.reset_text.trim();
  return text === "" ? null : text;
}

/** "in 4m" style wait used by rate-limit copy; `null` once the time has passed. */
export function formatWaitUntil(iso: string | null, now: number): string | null {
  const target = parseTime(iso);
  if (target === null || target <= now) return null;
  return formatDurationMinutes(Math.floor((target - now) / MINUTE_MS));
}

/** "just now", "2 min ago", "3 hours ago", "2 days ago". */
export function formatAgo(iso: string | null, now: number): string | null {
  const target = parseTime(iso);
  if (target === null) return null;
  const minutes = Math.floor(Math.max(0, now - target) / MINUTE_MS);
  if (minutes < 1) return "just now";
  if (minutes < HOUR_MINUTES) return `${minutes} min ago`;
  const hours = Math.floor(minutes / HOUR_MINUTES);
  if (hours < 24) return hours === 1 ? "1 hour ago" : `${hours} hours ago`;
  const days = Math.floor(hours / 24);
  return days === 1 ? "1 day ago" : `${days} days ago`;
}

/** Header/footer freshness: "Updated 2 min ago". */
export function formatUpdated(iso: string | null, now: number): string | null {
  const ago = formatAgo(iso, now);
  return ago === null ? null : `Updated ${ago}`;
}

const CURRENCY_CODE = /^[A-Z]{3}$/;
const UNIT_VALUE_FORMAT = new Intl.NumberFormat("en-US", { maximumFractionDigits: 2 });
const currencyFormats = new Map<string, Intl.NumberFormat | null>();

function currencyFormat(code: string): Intl.NumberFormat | null {
  let format = currencyFormats.get(code);
  if (format === undefined) {
    try {
      format = new Intl.NumberFormat("en-US", { style: "currency", currency: code });
    } catch {
      format = null;
    }
    currencyFormats.set(code, format);
  }
  return format;
}

/**
 * An amount in its native unit: ISO currencies render as money ("$12.40"),
 * everything else keeps its unit ("120 credits"). Amounts are never converted
 * or summed across units.
 */
export function formatAmount(amount: ProviderUsageAmount): string {
  const numeric = Number(amount.value);
  if (!Number.isFinite(numeric)) return `${amount.value} ${amount.unit}`.trim();
  if (CURRENCY_CODE.test(amount.unit)) {
    return currencyFormat(amount.unit)?.format(numeric) ?? `${amount.value} ${amount.unit}`;
  }
  const value = UNIT_VALUE_FORMAT.format(numeric);
  return amount.unit === "" ? value : `${value} ${amount.unit}`;
}

/** Calendar date for expiries and measured times ("Oct 28"). */
export function formatShortDate(iso: string | null): string | null {
  const target = parseTime(iso);
  return target === null ? null : RESET_DATE_FORMAT.format(target);
}

export function isOlderThan(iso: string | null, now: number, maxAgeMs: number): boolean {
  const target = parseTime(iso);
  return target === null || now - target > maxAgeMs;
}

export function compareResetsAt(left: string | null, right: string | null): number {
  const a = parseTime(left);
  const b = parseTime(right);
  if (a === b) return 0;
  if (a === null) return 1;
  if (b === null) return -1;
  return a - b;
}

/** Provider plan strings are verbatim ("max_20x"); display them as "Max 20x". */
export function formatPlan(plan: string): string {
  const words = plan.trim().replaceAll(/[_-]+/g, " ");
  return words === "" ? "" : words.charAt(0).toUpperCase() + words.slice(1);
}
