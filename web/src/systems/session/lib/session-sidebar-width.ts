/**
 * Session-window rail width, in CSS pixels. The default is the rail's
 * historical fixed width (`w-66`); the bounds keep two-line rows legible at
 * the narrow end and stop the rail from becoming the window at the wide end.
 * The transcript's own floor lives with the layout (half the window).
 */
export const SESSION_SIDEBAR_DEFAULT_WIDTH = 264;
export const SESSION_SIDEBAR_MIN_WIDTH = 200;
export const SESSION_SIDEBAR_MAX_WIDTH = 480;

/**
 * A stored or measured width the rail can honor, or `null` when the value is
 * not a width at all — the caller then falls back to the default. Finite
 * numbers clamp into bounds and round to whole pixels.
 */
export function normalizeSessionSidebarWidth(value: unknown): number | null {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return null;
  }
  return Math.round(
    Math.min(SESSION_SIDEBAR_MAX_WIDTH, Math.max(SESSION_SIDEBAR_MIN_WIDTH, value))
  );
}
