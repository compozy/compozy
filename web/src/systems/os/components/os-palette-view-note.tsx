import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

export interface OsPaletteViewNoteProps {
  children: ReactNode;
  /** `empty` is the list body; `note` sits under results. */
  placement?: "empty" | "note";
}

/**
 * A palette view speaking about its own list — why it is empty, or how much of
 * it is off screen. Never selectable: ↑↓ and ⏎ stay reserved for real results.
 */
export function OsPaletteViewNote({ children, placement = "note" }: OsPaletteViewNoteProps) {
  return (
    <p
      data-slot="os-palette-view-note"
      className={cn(
        placement === "empty"
          ? "px-3 py-6 text-center text-small-body text-muted"
          : "px-3 py-1 text-micro text-subtle"
      )}
    >
      {children}
    </p>
  );
}
