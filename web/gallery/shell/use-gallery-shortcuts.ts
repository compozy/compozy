import { flushSync } from "react-dom";
import { useState, type RefObject } from "react";

import { useTheme } from "#gallery/render/use-theme";
import type { FrameInfo } from "#gallery/shell/use-frame";
import { matches, type Registry } from "#gallery/shell/registry-model";
import { step, type GallerySelection } from "#gallery/shell/use-selection";
import { useShortcuts, type ShortcutAction } from "#gallery/shell/use-shortcuts";
import { select, updateView } from "#gallery/shell/view";

const collapsedKey = "gallery-sidebar-collapsed";

/** Gallery-wide UI state that is not part of the URL. */
export function useGalleryState(registry: Registry) {
  const [query, setQuery] = useState("");
  const [helpOpen, setHelpOpen] = useState(false);
  const [frame, setFrame] = useState<FrameInfo | null>(null);
  /** Comment mode: the canvas picks an element or region to comment on. */
  const [picking, setPicking] = useState(false);
  /** Side panel of the flow canvas (step inspector and comments). */
  const [flowSide, setFlowSide] = useState(true);
  const [collapsed, setCollapsedState] = useState(
    () => localStorage.getItem(collapsedKey) === "true"
  );
  const setCollapsed = (next: boolean) => {
    localStorage.setItem(collapsedKey, String(next));
    setCollapsedState(next);
  };
  const navigable = registry.entries.filter(entry => matches(entry, query));
  return {
    query,
    setQuery,
    helpOpen,
    setHelpOpen,
    frame,
    setFrame,
    picking,
    setPicking,
    flowSide,
    setFlowSide,
    collapsed,
    setCollapsed,
    navigable,
  };
}

export type GalleryState = ReturnType<typeof useGalleryState>;

/**
 * J/K step through prototypes, [/] through states, T switches the theme, C toggles comment
 * mode, / searches, ? shows help and Cmd/Ctrl+B toggles the sidebar. Shortcuts also work with focus inside the canvas.
 */
export function useGalleryShortcuts(
  selection: GallerySelection,
  ui: GalleryState,
  searchRef: RefObject<HTMLInputElement | null>
) {
  const dark = useTheme() === "dark";
  const { current, states, state } = selection;

  const goTo = (direction: 1 | -1) => {
    const next = step(ui.navigable, current ?? undefined, direction);
    if (next) select({ kind: "prototype", slug: next.slug });
  };
  const stepState = (direction: 1 | -1) => {
    const next = step(
      states.map(option => option.id),
      state,
      direction
    );
    if (next) updateView({ state: next });
  };
  const search = () => {
    // The search lives in the sidebar: open it first when it is collapsed.
    flushSync(() => ui.setCollapsed(false));
    searchRef.current?.focus({ preventScroll: true });
  };

  const actions: Record<ShortcutAction, () => void> = {
    next: () => goTo(1),
    previous: () => goTo(-1),
    "next-state": () => stepState(1),
    "previous-state": () => stepState(-1),
    theme: () => updateView({ theme: dark ? "light" : "dark" }),
    comment: () => {
      if (selection.flow) {
        ui.setFlowSide(true);
        updateView({ comments: !selection.view.comments, comment: null });
      } else if (current && !selection.view.compare) ui.setPicking(!ui.picking);
    },
    search,
    help: () => ui.setHelpOpen(!ui.helpOpen),
    sidebar: () => ui.setCollapsed(!ui.collapsed),
  };
  useShortcuts(action => actions[action](), ui.frame?.doc.defaultView ?? null);
}
