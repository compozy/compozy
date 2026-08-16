import type { ReactNode } from "react";

export interface OsPaletteViewNoteProps {
  children: ReactNode;
}

/**
 * A palette view speaking about its own list — why it is empty, or how much of
 * it is off screen. Never selectable: ↑↓ and ⏎ stay reserved for real results.
 */
export function OsPaletteViewNote({ children }: OsPaletteViewNoteProps) {
  return (
    <p data-slot="os-palette-view-note" className="px-3 py-2 text-micro text-subtle">
      {children}
    </p>
  );
}
