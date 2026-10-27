/**
 * The roster's row geometry, shared by the live rows and their loading placeholders so the two
 * cannot drift: a 20 px avatar column beside the title, with the trail at the row end.
 */
export const SUBAGENT_ROSTER_LIST_FRAME =
  "-mx-1.5 flex max-h-79 flex-col gap-px overflow-auto pb-0.5";

export const SUBAGENT_ROSTER_ROW_FRAME =
  "group/roster-row grid grid-cols-[minmax(0,1fr)_auto] items-center gap-1 rounded-sm pr-1.5 text-fg hover:bg-surface-2 has-focus-visible:bg-surface-2";

export const SUBAGENT_ROSTER_MAIN_FRAME =
  "grid min-w-0 grid-cols-[20px_minmax(0,1fr)] gap-2 rounded-sm pl-1.5 text-left outline-none focus-visible:shadow-focus-inset";

/** The main control's height: a plain row, or one that grows a branch line under its title. */
export const SUBAGENT_ROSTER_MAIN_PLAIN = "min-h-7.5 items-center py-1";
export const SUBAGENT_ROSTER_MAIN_BRANCH = "min-h-10 items-start py-1.25";
