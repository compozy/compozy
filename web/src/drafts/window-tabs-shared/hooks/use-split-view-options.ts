import { createContext, use } from "react";

import type { DraftWindow, DropEdge } from "../model";

export type SeamState = "hover" | "drag" | "focus" | "limit";

export interface SplitViewOptions {
  windows: Record<string, DraftWindow>;
  focusedId?: string;
  /** More than one window in the tab: heads offer "Move to new tab". */
  inSplit: boolean;
  /** `${splitId}:${seamIndex}` → forced seam presentation. */
  seams?: Record<string, SeamState>;
  /** Window-edge drop zone under a dragged tab or head. */
  drop?: { windowId: string; edge: DropEdge; mode?: "split" | "swap" };
  /** The window whose head is being dragged. */
  draggingId?: string;
  /** Splits that cannot fit their members show one at a time. */
  stacked?: Record<string, string>;
  onShowMember?: (splitId: string, windowId: string) => void;
  onFocus?: (windowId: string) => void;
  onClose?: (windowId: string) => void;
  onUnsplit?: (windowId: string) => void;
  onHeadPointerDown?: (windowId: string, event: React.PointerEvent<HTMLElement>) => void;
  onSeamPointerDown?: (
    splitId: string,
    index: number,
    event: React.PointerEvent<HTMLElement>
  ) => void;
  onSeamBalance?: (splitId: string, index: number) => void;
}

export const SplitViewOptionsContext = createContext<SplitViewOptions | null>(null);

export function useSplitViewOptions(): SplitViewOptions {
  const options = use(SplitViewOptionsContext);
  if (!options) throw new Error("OsSplitView options missing");
  return options;
}
