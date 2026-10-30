/**
 * The board's session drawer width, in CSS pixels. The floor keeps the composer and its
 * controls on one row; the default leaves a column of the board beside the drawer in a
 * typical window; the ceiling stops the drawer from becoming the window (the drawer also
 * caps itself at 70% of the board).
 */
export const SESSION_BOARD_DRAWER_DEFAULT_WIDTH = 640;
export const SESSION_BOARD_DRAWER_MIN_WIDTH = 420;
export const SESSION_BOARD_DRAWER_MAX_WIDTH = 1200;

/**
 * Below this board width the drawer covers the board edge to edge: the drawer (at most 70% of
 * the board) would leave less than one board column beside it.
 */
export const SESSION_BOARD_DRAWER_INLINE_BREAKPOINT = 960;

/** A stored or dragged width the drawer can honor, or `null` when the value is not a width. */
export function normalizeSessionBoardDrawerWidth(value: unknown): number | null {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return null;
  }
  return Math.round(
    Math.min(SESSION_BOARD_DRAWER_MAX_WIDTH, Math.max(SESSION_BOARD_DRAWER_MIN_WIDTH, value))
  );
}
