/**
 * The sessions split's geometry, shared by the live layout and the static frame a window shows
 * before its code has loaded, so the two cannot drift: the row the rail, its handle and the pane
 * share, and the pane right of the rail. Free of imports so the static frame does not pull the
 * panel library in with the live layout.
 */
export const SESSION_SIDEBAR_SPLIT_FRAME = "min-h-0 min-w-0 flex-1";

export const SESSION_SIDEBAR_PANE_FRAME = "flex min-w-0";
