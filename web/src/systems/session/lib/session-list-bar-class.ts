/**
 * The strip both session-list bars share. The selection bar swaps in for this
 * toolbar, so the height is pinned (a default button plus the vertical padding)
 * rather than left to whichever control happens to be tallest: the filter below
 * must not move when a selection starts.
 */
export const SESSION_LIST_BAR_CLASS =
  "flex min-h-[calc(var(--height-button-default)+var(--spacing)*3)] items-center gap-1 px-3 py-1.5";
