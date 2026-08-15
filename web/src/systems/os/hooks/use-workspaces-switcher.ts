import { useEffect, useEffectEvent, useLayoutEffect, useRef, useState } from "react";

import type { WorkspacesMenuNavRow } from "../lib/workspaces-overview-model";
import type { WorkspacePayload, WorkspaceTreeNode } from "@/systems/workspace";

/** Trailing dashed tile; matches the typeahead word "new". */
export const WORKSPACES_ADD_TILE_KEY = "__add";

interface WorkspaceSwitcherWorkspaceEntry {
  /** Workspace id, or `WORKSPACES_ADD_TILE_KEY` for the trailing add tile. */
  key: string;
  kind: "workspace";
  node: WorkspaceTreeNode<WorkspacePayload>;
}

interface WorkspaceSwitcherAddEntry {
  key: typeof WORKSPACES_ADD_TILE_KEY;
  kind: "add";
  node: null;
}

export type WorkspacesSwitcherEntry = WorkspaceSwitcherWorkspaceEntry | WorkspaceSwitcherAddEntry;

export type WorkspacesSwitcherLayer = "strip" | "menu";

export interface UseWorkspacesSwitcherInput {
  entries: readonly WorkspacesSwitcherEntry[];
  /** Nav rows per entry key (inert rows already excluded). Pure lookup. */
  menuNavRowsForKey: (key: string) => readonly WorkspacesMenuNavRow[];
  /** Where focus lands on open — the current workspace, or the first tile. */
  initialFocusKey: string | null;
  /** Menu row carrying the scope check; `↓` enters the menu on this row. */
  scopedRowKeyForKey: (key: string) => string | null;
  reducedMotion: boolean;
  onActivateEntry: (entry: WorkspacesSwitcherEntry) => void;
  onActivateMenuRow: (entry: WorkspacesSwitcherEntry, row: WorkspacesMenuNavRow) => void;
  /** Scrolls the focused tile clear of the edge fades. */
  keepInView: (element: HTMLElement, options?: { instant?: boolean }) => void;
  /** Suppresses the synthetic click that trails a drag flick. */
  consumeDragClick: () => boolean;
  /** Hover-focus is suppressed while the track is being dragged. */
  isTrackDragging: () => boolean;
}

export interface WorkspacesSwitcher {
  layer: WorkspacesSwitcherLayer;
  focusIndex: number;
  menuIndex: number;
  focusedEntry: WorkspacesSwitcherEntry | undefined;
  registerTile: (key: string) => (element: HTMLElement | null) => void;
  registerRow: (key: string) => (element: HTMLElement | null) => void;
  onStageKeyDown: (event: React.KeyboardEvent) => void;
  tileHandlers: (index: number) => {
    onClick: () => void;
    onMouseMove: () => void;
  };
  menuRowHandlers: (navIndex: number) => { onClick: () => void };
  /** Scroll settle / drag release — focus the nearest tile without scrolling. */
  focusNearest: (index: number) => void;
  /**
   * Escape ladder step for the dialog's close request: `true` consumed the
   * escape by leaving the menu layer; `false` lets the overlay close.
   */
  guardEscape: () => boolean;
}

const TYPEAHEAD_RESET_MS = 650;
function isTypeaheadKey(key: string): boolean {
  return Array.from(key).length === 1 && !/^\s$/u.test(key);
}

interface FocusIntent {
  instant?: boolean;
  skipScroll?: boolean;
}

interface FocusPosition {
  key: string | null;
  /** Nearest surviving position when the keyed entry disappears. */
  index: number;
}

function typeaheadName(entry: WorkspacesSwitcherEntry): string {
  return entry.kind === "add" ? "new" : entry.node.workspace.name.toLowerCase();
}

/**
 * Command-Tab interaction model: a roving-tabindex strip layer over a vertical
 * worktree-menu layer. Real DOM focus is the single focus model — there is no
 * aria-activedescendant mirror (workspaces DESIGN-NOTES keyboard contract).
 * Handlers move DOM focus synchronously. A callback ref owns initial focus;
 * the layout effect reconciles keyed focus when live entries change.
 */
export function useWorkspacesSwitcher({
  entries,
  menuNavRowsForKey,
  initialFocusKey,
  scopedRowKeyForKey,
  reducedMotion,
  onActivateEntry,
  onActivateMenuRow,
  keepInView,
  consumeDragClick,
  isTrackDragging,
}: UseWorkspacesSwitcherInput): WorkspacesSwitcher {
  const [stripFocus, setStripFocusPosition] = useState<FocusPosition>(() => {
    const initialIndex = entries.findIndex(entry => entry.key === initialFocusKey);
    const index = initialIndex >= 0 ? initialIndex : 0;
    return { key: entries[index]?.key ?? null, index };
  });
  const [layer, setLayer] = useState<WorkspacesSwitcherLayer>("strip");
  const [menuFocus, setMenuFocusPosition] = useState<FocusPosition>({ key: null, index: 0 });

  const tileElsRef = useRef(new Map<string, HTMLElement>());
  const rowElsRef = useRef(new Map<string, HTMLElement>());
  const typeBufferRef = useRef("");
  const typeTimerRef = useRef(0);
  const storedFocusIndex = entries.findIndex(entry => entry.key === stripFocus.key);
  const initialEntryIndex =
    stripFocus.key === null ? entries.findIndex(entry => entry.key === initialFocusKey) : -1;
  const focusIndex =
    storedFocusIndex >= 0
      ? storedFocusIndex
      : initialEntryIndex >= 0
        ? initialEntryIndex
        : Math.min(stripFocus.index, Math.max(0, entries.length - 1));
  const focusedEntry = entries[focusIndex];
  const menuNavRows = focusedEntry ? menuNavRowsForKey(focusedEntry.key) : [];
  const storedMenuIndex = menuNavRows.findIndex(row => row.key === menuFocus.key);
  const menuIndex =
    storedMenuIndex >= 0
      ? storedMenuIndex
      : Math.min(menuFocus.index, Math.max(0, menuNavRows.length - 1));
  const focusedMenuRow = menuNavRows[menuIndex];
  const activeLayer = layer === "menu" && menuNavRows.length === 0 ? "strip" : layer;

  if (activeLayer !== layer) setLayer(activeLayer);
  if (activeLayer === "menu" && focusedMenuRow && menuFocus.key !== focusedMenuRow.key) {
    setMenuFocusPosition({ key: focusedMenuRow.key, index: menuIndex });
  }
  if (activeLayer === "strip" && focusedEntry && stripFocus.key !== focusedEntry.key) {
    setStripFocusPosition({ key: focusedEntry.key, index: focusIndex });
  }

  const focusTileElement = (key: string | undefined, intent: FocusIntent) => {
    const element = key ? tileElsRef.current.get(key) : undefined;
    if (!element) return;
    element.focus({ preventScroll: true });
    if (!intent.skipScroll) keepInView(element, { instant: intent.instant ?? reducedMotion });
  };

  const setStripFocus = (index: number, intent: FocusIntent = {}) => {
    setLayer("strip");
    const key = entries[index]?.key;
    setStripFocusPosition({ key: key ?? null, index });
    focusTileElement(key, intent);
  };

  const setMenuFocus = (rows: readonly WorkspacesMenuNavRow[], navIndex: number) => {
    const row = rows[navIndex];
    if (!row) return;
    setLayer("menu");
    setMenuFocusPosition({ key: row.key, index: navIndex });
    const element = rowElsRef.current.get(row.key);
    // preventScroll keeps the overlay page still; the nest scrolls its own
    // viewport so rows past the menu's cap stay reachable by arrow keys.
    element?.focus({ preventScroll: true });
    element?.scrollIntoView({ block: "nearest" });
  };

  const reconcileDOMFocus = useEffectEvent(
    (element: HTMLElement, resolvedLayer: WorkspacesSwitcherLayer) => {
      element.focus({ preventScroll: true });
      if (resolvedLayer === "strip") {
        keepInView(element, { instant: reducedMotion });
      } else {
        // Menu rows live inside the nest's own scroll viewport.
        element.scrollIntoView({ block: "nearest" });
      }
    }
  );

  useLayoutEffect(() => {
    const resolvedKey = activeLayer === "menu" ? focusedMenuRow?.key : focusedEntry?.key;
    if (!resolvedKey) return;
    const element =
      activeLayer === "menu"
        ? rowElsRef.current.get(resolvedKey)
        : tileElsRef.current.get(resolvedKey);
    if (element && document.activeElement !== element) reconcileDOMFocus(element, activeLayer);
  }, [activeLayer, focusedEntry?.key, focusedMenuRow?.key]);

  useEffect(() => {
    return () => window.clearTimeout(typeTimerRef.current);
  }, []);

  const moveStrip = (direction: 1 | -1) => {
    if (entries.length === 0) return;
    const next = (focusIndex + direction + entries.length) % entries.length;
    // Wrap jumps land instantly rather than scrubbing the whole strip.
    const wrapped = (direction > 0 && next < focusIndex) || (direction < 0 && next > focusIndex);
    setStripFocus(next, { instant: wrapped });
  };

  const enterMenu = () => {
    if (!focusedEntry || menuNavRows.length === 0) return;
    const scopedKey = scopedRowKeyForKey(focusedEntry.key);
    const scoped = scopedKey ? menuNavRows.findIndex(row => row.key === scopedKey) : -1;
    setMenuFocus(menuNavRows, scoped >= 0 ? scoped : 0);
  };

  const leaveMenu = () => {
    setStripFocus(focusIndex, { skipScroll: true });
  };

  const moveMenu = (direction: 1 | -1) => {
    if (menuNavRows.length === 0) return;
    const next = menuIndex + direction;
    if (next < 0) {
      leaveMenu();
      return;
    }
    setMenuFocus(menuNavRows, Math.min(menuNavRows.length - 1, next));
  };

  const activate = () => {
    if (activeLayer === "menu") {
      const row = menuNavRows[menuIndex];
      if (focusedEntry && row) onActivateMenuRow(focusedEntry, row);
      return;
    }
    if (focusedEntry) onActivateEntry(focusedEntry);
  };

  const typeJump = (char: string) => {
    window.clearTimeout(typeTimerRef.current);
    typeTimerRef.current = window.setTimeout(() => {
      typeBufferRef.current = "";
    }, TYPEAHEAD_RESET_MS);
    const buffer = typeBufferRef.current;
    // Repeating one letter cycles its matches instead of pinning the first.
    const cycling = buffer.length > 0 && [...buffer].every(existing => existing === char);
    typeBufferRef.current = buffer + char;
    const prefix = cycling ? char : typeBufferRef.current;
    const start = cycling ? focusIndex + 1 : focusIndex;
    for (let step = 0; step < entries.length; step++) {
      const index = (start + step) % entries.length;
      const entry = entries[index];
      if (entry && typeaheadName(entry).startsWith(prefix)) {
        if (index !== focusIndex || activeLayer !== "strip") setStripFocus(index);
        return;
      }
    }
  };

  const onStageKeyDown = (event: React.KeyboardEvent) => {
    const target = event.target;
    if (
      target instanceof Element &&
      target.closest('[data-slot="dropdown-menu-trigger"], [data-slot="dropdown-menu-content"]')
    ) {
      return;
    }
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    switch (event.key) {
      case "ArrowRight":
        event.preventDefault();
        moveStrip(1);
        return;
      case "ArrowLeft":
        event.preventDefault();
        moveStrip(-1);
        return;
      case "ArrowDown":
        event.preventDefault();
        if (activeLayer === "menu") moveMenu(1);
        else enterMenu();
        return;
      case "ArrowUp":
        event.preventDefault();
        if (activeLayer === "menu") moveMenu(-1);
        return;
      case "Home":
        event.preventDefault();
        if (activeLayer === "menu") setMenuFocus(menuNavRows, 0);
        else if (entries.length > 0) setStripFocus(0, { instant: true });
        return;
      case "End":
        event.preventDefault();
        if (activeLayer === "menu") setMenuFocus(menuNavRows, menuNavRows.length - 1);
        else if (entries.length > 0) setStripFocus(entries.length - 1, { instant: true });
        return;
      case "Enter":
      case " ":
        if (!focusedEntry && target instanceof Element && target.closest("button")) return;
        event.preventDefault();
        activate();
        return;
      case "Tab":
        // Focus is trapped in the overlay; Tab is remapped to movement.
        event.preventDefault();
        if (activeLayer === "menu") moveMenu(event.shiftKey ? -1 : 1);
        else moveStrip(event.shiftKey ? -1 : 1);
        return;
      default:
    }
    if (isTypeaheadKey(event.key)) {
      event.preventDefault();
      typeJump(event.key.toLowerCase());
    }
  };

  const registerTile = (key: string) => (element: HTMLElement | null) => {
    if (!element) {
      tileElsRef.current.delete(key);
      return;
    }
    tileElsRef.current.set(key, element);
  };

  const registerRow = (key: string) => (element: HTMLElement | null) => {
    if (element) rowElsRef.current.set(key, element);
    else rowElsRef.current.delete(key);
  };

  const tileHandlers = (index: number) => ({
    onClick: () => {
      if (consumeDragClick()) return;
      setStripFocus(index, { skipScroll: true });
      const entry = entries[index];
      if (entry) onActivateEntry(entry);
    },
    onMouseMove: () => {
      if (isTrackDragging()) return;
      if (activeLayer === "strip" && index === focusIndex) return;
      setStripFocus(index, { skipScroll: true });
    },
  });

  const menuRowHandlers = (navIndex: number) => ({
    onClick: () => {
      const row = menuNavRows[navIndex];
      if (!focusedEntry || !row) return;
      setMenuFocus(menuNavRows, navIndex);
      onActivateMenuRow(focusedEntry, row);
    },
  });

  const focusNearest = (index: number) => {
    if (activeLayer === "strip" && index === focusIndex) return;
    setStripFocus(index, { skipScroll: true });
  };

  const guardEscape = () => {
    if (activeLayer !== "menu") return false;
    leaveMenu();
    return true;
  };

  return {
    layer: activeLayer,
    focusIndex,
    menuIndex,
    focusedEntry,
    registerTile,
    registerRow,
    onStageKeyDown,
    tileHandlers,
    menuRowHandlers,
    focusNearest,
    guardEscape,
  };
}
