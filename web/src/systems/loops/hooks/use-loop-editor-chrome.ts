import {
  SIDEBAR_COLLAPSE_BREAKPOINT_DEFAULT,
  SIDEBAR_PANEL_WIDTH_MD_BREAKPOINT,
  useSidebarViewport,
} from "@compozy/ui";

import type { LoopEditorPaletteMode } from "../lib/loop-editor-types";
import {
  useLoopEditorChromeState,
  type UseLoopEditorChromeStateResult,
} from "./use-loop-editor-chrome-state";

export interface LoopEditorChrome extends UseLoopEditorChromeStateResult {
  paletteMode: LoopEditorPaletteMode;
}

export function useLoopEditorChrome(): LoopEditorChrome {
  const chrome = useLoopEditorChromeState();
  const viewport = useSidebarViewport({
    drawer: SIDEBAR_COLLAPSE_BREAKPOINT_DEFAULT,
    md: SIDEBAR_PANEL_WIDTH_MD_BREAKPOINT,
  });
  const paletteMode: LoopEditorPaletteMode =
    viewport === "drawer" ? "menu" : chrome.paletteOpen ? "expanded" : "collapsed";
  return { ...chrome, paletteMode };
}
