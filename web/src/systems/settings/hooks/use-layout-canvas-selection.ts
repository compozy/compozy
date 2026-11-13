import { useState } from "react";

import { findLayoutLeaf } from "../lib/window-manager-layout-tree";
import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutTab,
} from "../lib/window-manager-layout-types";

interface LayoutSelectionState {
  tabId: string | null;
  windowId: string | null;
}

export interface LayoutCanvasSelectionModel {
  /** The tab on the canvas: the one picked, else the desktop's default tab. */
  tab: WindowManagerLayoutTab | null;
  /** The window open in the inspector; always a member of `tab`. */
  windowId: string | null;
  selectTab: (tabId: string) => void;
  selectWindow: (windowId: string | null) => void;
  /** Shows a tab and one of its windows together, as after a move. */
  reveal: (tabId: string, windowId: string | null) => void;
}

/**
 * Selection survives edits but not deletions: closing a tab, or moving a
 * window out of the one on screen, drops back to what still exists rather than
 * pointing the inspector at something the document no longer contains.
 */
export function useLayoutCanvasSelection(
  desktop: WindowManagerLayoutDesktop | undefined
): LayoutCanvasSelectionModel {
  const [state, setState] = useState<LayoutSelectionState>({ tabId: null, windowId: null });
  const tab =
    desktop?.tabs.find(candidate => candidate.id === state.tabId) ??
    desktop?.tabs.find(candidate => candidate.id === desktop.activeTabId) ??
    desktop?.tabs[0] ??
    null;
  const windowId =
    tab !== null && state.windowId !== null && findLayoutLeaf(tab.root, state.windowId) !== null
      ? state.windowId
      : null;

  return {
    tab,
    windowId,
    selectTab: tabId => setState({ tabId, windowId: null }),
    selectWindow: next => setState({ tabId: tab?.id ?? null, windowId: next }),
    reveal: (tabId, next) => setState({ tabId, windowId: next }),
  };
}
