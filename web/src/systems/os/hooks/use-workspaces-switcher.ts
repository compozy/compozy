import { useEffect, useRef, useState } from "react";

import type { WorkspacePayload, WorkspaceTreeNode, WorktreeNestEntry } from "@/systems/workspace";

/** Trailing dashed tile; matches the typeahead word "new". */
export const WORKSPACES_ADD_TILE_KEY = "__add";

export interface WorkspacesSwitcherEntry {
  /** Workspace id, or `WORKSPACES_ADD_TILE_KEY` for the trailing add tile. */
  key: string;
  kind: "workspace" | "add";
  node: WorkspaceTreeNode<WorkspacePayload> | null;
}

/** Arrow-reachable rows of the focused workspace's worktree menu. */
export interface WorkspacesMenuNavRow {
  key: string;
  kind: "worktree" | "create";
  entry: WorktreeNestEntry | null;
}

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
const TYPEAHEAD_PATTERN = /^[a-z0-9-]$/i;

interface FocusIntent {
  instant?: boolean;
  skipScroll?: boolean;
}

function typeaheadName(entry: WorkspacesSwitcherEntry): string {
  return entry.kind === "add" ? "new" : (entry.node?.workspace.name.toLowerCase() ?? "");
}

/**
 * Command-Tab interaction model: a roving-tabindex strip layer over a vertical
 * worktree-menu layer. Real DOM focus is the single focus model — there is no
 * aria-activedescendant mirror (workspaces DESIGN-NOTES keyboard contract).
 * Handlers move DOM focus synchronously; the one effect is the initial focus
 * on open.
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
  const [focusIndex, setFocusIndex] = useState(() => {
    const index = entries.findIndex(entry => entry.key === initialFocusKey);
    return index >= 0 ? index : 0;
  });
  const [layer, setLayer] = useState<WorkspacesSwitcherLayer>("strip");
  const [menuIndex, setMenuIndex] = useState(0);

  const tileElsRef = useRef(new Map<string, HTMLElement>());
  const rowElsRef = useRef(new Map<string, HTMLElement>());
  const typeBufferRef = useRef("");
  const typeTimerRef = useRef(0);
  const initialFocusDoneRef = useRef(false);

  const focusedEntry = entries[focusIndex];
  const menuNavRows = focusedEntry ? menuNavRowsForKey(focusedEntry.key) : [];

  const focusTileElement = (key: string | undefined, intent: FocusIntent) => {
    const element = key ? tileElsRef.current.get(key) : undefined;
    if (!element) return;
    element.focus({ preventScroll: true });
    if (!intent.skipScroll) keepInView(element, { instant: intent.instant ?? reducedMotion });
  };

  const setStripFocus = (index: number, intent: FocusIntent = {}) => {
    setLayer("strip");
    setFocusIndex(index);
    focusTileElement(entries[index]?.key, intent);
  };

  const setMenuFocus = (rows: readonly WorkspacesMenuNavRow[], navIndex: number) => {
    const row = rows[navIndex];
    if (!row) return;
    setLayer("menu");
    setMenuIndex(navIndex);
    rowElsRef.current.get(row.key)?.focus({ preventScroll: true });
  };

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
    if (layer === "menu") {
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
        if (index !== focusIndex || layer !== "strip") setStripFocus(index);
        return;
      }
    }
  };

  const onStageKeyDown = (event: React.KeyboardEvent) => {
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
        if (layer === "menu") moveMenu(1);
        else enterMenu();
        return;
      case "ArrowUp":
        event.preventDefault();
        if (layer === "menu") moveMenu(-1);
        return;
      case "Home":
        event.preventDefault();
        if (layer === "menu") setMenuFocus(menuNavRows, 0);
        else if (entries.length > 0) setStripFocus(0, { instant: true });
        return;
      case "End":
        event.preventDefault();
        if (layer === "menu") setMenuFocus(menuNavRows, menuNavRows.length - 1);
        else if (entries.length > 0) setStripFocus(entries.length - 1, { instant: true });
        return;
      case "Enter":
      case " ":
        event.preventDefault();
        activate();
        return;
      case "Tab":
        // Focus is trapped in the overlay; Tab is remapped to movement.
        event.preventDefault();
        if (layer === "menu") moveMenu(event.shiftKey ? -1 : 1);
        else moveStrip(event.shiftKey ? -1 : 1);
        return;
      default:
    }
    if (event.key.length === 1 && TYPEAHEAD_PATTERN.test(event.key)) {
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
    // Initial focus: the overlay mounts fresh per open and focus must land on
    // the current workspace's tile, kept clear of the edge fades. The ref
    // callback is the imperative DOM slot for this one-time commit-time sync.
    if (!initialFocusDoneRef.current && entries[focusIndex]?.key === key) {
      initialFocusDoneRef.current = true;
      element.focus({ preventScroll: true });
      keepInView(element, { instant: reducedMotion });
    }
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
      if (layer === "strip" && index === focusIndex) return;
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
    if (layer === "strip" && index === focusIndex) return;
    setStripFocus(index, { skipScroll: true });
  };

  const guardEscape = () => {
    if (layer !== "menu") return false;
    leaveMenu();
    return true;
  };

  return {
    layer,
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
