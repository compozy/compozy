/**
 * The roster's row geometry, shared by the live rows and their loading placeholders so the two
 * cannot drift: a 32 px tile column beside the text, with the trail at the row end.
 */
export const SUBAGENT_ROSTER_LIST_FRAME =
  "-mx-1.5 flex max-h-79 flex-col gap-0.5 overflow-auto pb-0.5";

export const SUBAGENT_ROSTER_ROW_FRAME =
  "group/roster-row grid grid-cols-[minmax(0,1fr)_auto] items-start gap-2 rounded-lg pr-2 text-fg transition-colors duration-fast ease-out hover:bg-surface-2 has-focus-visible:bg-surface-2";

export const SUBAGENT_ROSTER_MAIN_FRAME =
  "grid min-h-12 min-w-0 grid-cols-[32px_minmax(0,1fr)] items-start gap-3 rounded-lg py-2 pl-2 text-left outline-none focus-visible:shadow-focus-inset";

export const SUBAGENT_ROSTER_FIRST_PAGE = 6;
