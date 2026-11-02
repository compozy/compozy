/** The recessed strip the tabs sit in, shared by the live deck and hosts that compose the view. */
export const OS_WINDOW_DECK_STRIP_CLASS =
  "flex h-deck shrink-0 items-end bg-sunken pr-1 shadow-[inset_0_-1px_0_var(--color-line)] select-none";

/** One tab's slot: its width and the hairline on its right edge. */
export const OS_WINDOW_DECK_SLOT_CLASS =
  "relative flex after:absolute after:inset-y-2.25 after:right-0 after:w-px after:bg-line last-of-type:after:opacity-0 hover:after:opacity-0 data-active:after:opacity-0 has-[+[data-deck-slot]:hover]:after:opacity-0 has-[+[data-active]]:after:opacity-0";
