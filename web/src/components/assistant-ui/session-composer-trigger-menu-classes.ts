import { cn } from "@/lib/utils";

/** Class contracts shared by the composer trigger popovers (`/` and `@`). */

/** The popover surface above the editor; positioned by the composer stack. */
export const TRIGGER_MENU_POPOVER_CLASS = cn(
  "absolute inset-x-0 bottom-full z-20 mb-2 overflow-hidden",
  "rounded-lg bg-popover shadow-pop"
);

/** The scrolling list inside the popover. */
export const TRIGGER_MENU_ITEMS_CLASS = "max-h-72 overflow-y-auto overscroll-contain p-1";

/**
 * The geometry of a trigger-menu row, shared by the live row and its loading placeholder so the
 * two cannot drift: a 16 px icon well, then the title and its secondary text on one baseline.
 */
export const TRIGGER_MENU_ROW_GEOMETRY =
  "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-small-body";
export const TRIGGER_MENU_ROW_ICON_WELL = "flex size-4 shrink-0 items-center justify-center";
export const TRIGGER_MENU_ROW_LABEL = "flex min-w-0 flex-1 items-baseline gap-1.5";
