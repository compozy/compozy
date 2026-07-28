import { useState } from "react";

import { findLayoutNode } from "../lib/window-manager-layout-tree";
import type { WindowManagerLayoutDesktop } from "../lib/window-manager-layout-types";

export type LayoutSelection =
  | { kind: "group"; id: string }
  | { kind: "node"; id: string }
  | { kind: "window"; id: string }
  | null;

export interface LayoutCanvasSelectionModel {
  selection: LayoutSelection;
  select: (next: LayoutSelection) => void;
  clear: () => void;
}

/**
 * Selection survives edits but not deletions: converting a split to a stack
 * replaces the node ids underneath it, and pointing the inspector at something
 * the document no longer contains is how an editor starts lying.
 */
export function useLayoutCanvasSelection(
  desktop: WindowManagerLayoutDesktop
): LayoutCanvasSelectionModel {
  const [selection, setSelection] = useState<LayoutSelection>(null);

  const resolved = resolveSelection(selection, desktop);
  if (resolved !== selection) setSelection(resolved);

  return {
    selection: resolved,
    select: setSelection,
    clear: () => setSelection(null),
  };
}

function resolveSelection(
  selection: LayoutSelection,
  desktop: WindowManagerLayoutDesktop
): LayoutSelection {
  if (selection === null) return null;
  if (selection.kind === "node") {
    return findLayoutNode(desktop, selection.id) === null ? null : selection;
  }
  if (selection.kind === "group") {
    return desktop.groups.some(group => group.id === selection.id) ? selection : null;
  }
  return desktop.floating.includes(selection.id) ? selection : null;
}
