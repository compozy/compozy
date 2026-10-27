/**
 * The geometry of a session row, shared by the live row and its loading placeholder so the two
 * cannot drift: a 12 px mark column, the title column and the end slot on line 1, with the
 * who-line under the title on line 2.
 */
export const SESSION_ROW_FRAME =
  "relative grid grid-cols-[12px_minmax(0,1fr)_auto] items-center gap-x-2 gap-y-0.75 rounded-md px-2 py-1.5";

/** Line 1's title target: the status mark column beside the title. */
export const SESSION_ROW_TARGET_FRAME =
  "col-span-2 col-start-1 row-start-1 grid min-w-0 grid-cols-[12px_minmax(0,1fr)] items-center gap-x-2 text-left text-body text-fg";

/** Line 1's end slot: the compact time at rest, the row actions on hover or focus. */
export const SESSION_ROW_END_SLOT_FRAME =
  "col-start-3 row-start-1 grid min-w-6 items-center justify-items-end *:col-start-1 *:row-start-1";

/** Line 2: where and who, with the trailing cluster after it. */
export const SESSION_ROW_META_FRAME =
  "col-span-2 col-start-2 row-start-2 flex min-w-0 items-center gap-2";

/** The who-line itself: provider mark, agent and notes, clipped rather than wrapped. */
export const SESSION_ROW_WHO_FRAME =
  "flex min-w-0 flex-1 items-center gap-1.25 overflow-hidden text-eyebrow whitespace-nowrap text-subtle";

/** A subagent row on its parent's rail: status mark, title and trail, the same grid as a session row's first line. */
export const SUBAGENT_ROW_FRAME =
  "grid w-full grid-cols-[12px_minmax(0,1fr)_auto] items-center gap-x-2 rounded-md px-2 py-1.5 text-left text-body text-fg outline-none";

/** The who-line's trailing cluster (the branch toggle), lifted above the row target. */
export const SESSION_ROW_TRAILING_FRAME =
  "relative z-10 -mr-0.5 flex shrink-0 items-center gap-0.5";
