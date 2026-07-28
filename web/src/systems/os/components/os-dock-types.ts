import type { ComponentType } from "react";

export interface OsDockItemData {
  /** Stable app key. */
  id: string;
  /** Accessible app name (also the tooltip). */
  name: string;
  /** Glyph component — OpenDesign dock strokes via `DockIcons`, or Lucide. */
  icon: ComponentType<{ className?: string }>;
  /** Window is open. */
  running?: boolean;
  /** Window is minimized into its icon (hollow indicator, dimmed glyph). */
  minimized?: boolean;
  /** Attention count from a runtime projection; 0/undefined renders nothing. */
  badge?: number;
}

/** Group break matching OpenDesign `dock-sep` (sidebar group seams). */
export type OsDockSeparator = { id: string; sep: true };

export type OsDockEntry = OsDockItemData | OsDockSeparator;

export function isOsDockSeparator(entry: OsDockEntry): entry is OsDockSeparator {
  return "sep" in entry && entry.sep === true;
}
