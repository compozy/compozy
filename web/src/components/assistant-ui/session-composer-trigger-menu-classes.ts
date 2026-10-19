import { cn } from "@/lib/utils";

/** Class contracts shared by the composer trigger popovers (`/` and `@`). */

/** The popover surface above the editor; positioned by the composer stack. */
export const TRIGGER_MENU_POPOVER_CLASS = cn(
  "absolute inset-x-0 bottom-full z-20 mb-2 overflow-hidden",
  "rounded-lg bg-popover shadow-pop"
);

/** The scrolling list inside the popover. */
export const TRIGGER_MENU_ITEMS_CLASS = "max-h-72 overflow-y-auto overscroll-contain p-1";
