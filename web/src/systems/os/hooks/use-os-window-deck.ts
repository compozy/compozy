import { useSelector } from "@xstate/store-react";
import * as React from "react";

import { useActiveWorkspace } from "@/systems/workspace";
import { useSessions, type SessionPayload } from "@/systems/session";

import type { OsWindowFrameModel } from "../lib/group-projection";
import type { OsRect } from "../lib/os-types";
import { windowManagerStore } from "../stores/window-manager-store";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";

const DRAG_START_SLOP = 4;
const DECK_EXIT_SLOP = 28;
const EDGE_SCROLL_BAND = 24;
const EDGE_SCROLL_STEP = 12;
const EMPTY_TAB_ELEMENTS: ReadonlyMap<string, HTMLElement> = new Map();

export interface OsDeckTabDrag {
  windowId: string;
  mode: "reorder" | "out";
  /** Insertion slot (0..members.length) while reordering; null once outside. */
  insertIndex: number | null;
}

export interface OsWindowDeckModel {
  sessionById: ReadonlyMap<string, SessionPayload>;
  tabDrag: OsDeckTabDrag | null;
  /** Insertion slot advertised to the active window drag over this deck. */
  dropIndex: number | null;
  otherWindowCount: number;
  tabsRef: React.RefObject<HTMLDivElement | null>;
  registerTab: (windowId: string, element: HTMLElement | null) => void;
  handleTabPointerDown: (windowId: string, event: React.PointerEvent<HTMLElement>) => void;
  wasDragged: () => boolean;
  activateTab: (windowId: string) => void;
  closeTab: (windowId: string) => void;
  closeOthers: (windowId: string) => void;
  closeRight: (windowId: string) => void;
  pinTab: (windowId: string, pinned: boolean) => void;
  moveToNewWindow: (windowId: string) => void;
  mergeAllWindows: () => void;
  openNewTab: () => void;
}

function clampInsertToRegion(pinned: boolean, slot: number, pinnedCount: number): number {
  return pinned ? Math.min(slot, pinnedCount) : Math.max(slot, pinnedCount);
}

function insertSlotAt(
  members: readonly string[],
  tabElements: ReadonlyMap<string, HTMLElement>,
  clientX: number
): number {
  let slot = 0;
  for (const member of members) {
    const element = tabElements.get(member);
    if (!element) continue;
    const box = element.getBoundingClientRect();
    if (clientX > box.left + box.width / 2) slot += 1;
  }
  return slot;
}

function autoScrollAtEdges(tabs: HTMLDivElement | null, clientX: number): void {
  if (!tabs) return;
  const box = tabs.getBoundingClientRect();
  if (clientX < box.left + EDGE_SCROLL_BAND) tabs.scrollLeft -= EDGE_SCROLL_STEP;
  else if (clientX > box.right - EDGE_SCROLL_BAND) tabs.scrollLeft += EDGE_SCROLL_STEP;
}

function cancelActiveWindowGesture(): void {
  const gesture = windowManagerStore.getSnapshot().context.gesture;
  if (gesture?.status === "active") {
    windowManagerStore.trigger.gestureCancelled({ reason: "manual" });
  }
}

function updateDeckDropTarget({
  frame,
  tabs,
  tabElements,
  clientX,
  clientY,
}: {
  frame: OsWindowFrameModel;
  tabs: HTMLDivElement | null;
  tabElements: ReadonlyMap<string, HTMLElement>;
  clientX: number;
  clientY: number;
}): void {
  const gesture = windowManagerStore.getSnapshot().context.gesture;
  if (gesture?.status !== "active" || frame.members.includes(gesture.source.windowId) || !tabs) {
    windowManagerStore.trigger.deckDropCleared({ frameId: frame.id });
    return;
  }
  const box = tabs.getBoundingClientRect();
  const inside =
    clientX >= box.left &&
    clientX <= box.right &&
    clientY >= box.top - 4 &&
    clientY <= box.bottom + 8;
  if (!inside) {
    windowManagerStore.trigger.deckDropCleared({ frameId: frame.id });
    return;
  }
  autoScrollAtEdges(tabs, clientX);
  windowManagerStore.trigger.deckDropTargeted({
    target: {
      frameId: frame.id,
      targetWindowId: frame.activeWindowId,
      insertIndex: insertSlotAt(frame.members, tabElements, clientX),
    },
  });
}

/**
 * Deck view-model: shared session data for state dots, tab command dispatch,
 * and the pointer gestures — in-deck reorder, drag-out to a standalone window,
 * and advertising this deck as the merge target of an active window drag. The
 * store owns the drop-target fact; the drag session derives its commit from
 * the release point, so Escape and pointercancel commit nothing (US-013/014).
 */
export function useOsWindowDeck(frame: OsWindowFrameModel): OsWindowDeckModel {
  const { manager, coordinator } = useOsShell();
  const { activeWorkspaceId } = useActiveWorkspace();
  const memberIds = new Set(frame.members);
  const hasSessionMember = useDesktop(state =>
    frame.members.some(member => state.windows[member]?.app === "session")
  );
  const otherWindowCount = useDesktop(
    state =>
      Object.values(state.windows).filter(
        win => win.desktopId === frame.desktopId && !memberIds.has(win.id)
      ).length
  );
  const sessions = useSessions(activeWorkspaceId, {
    enabled: hasSessionMember && activeWorkspaceId !== null,
  });
  const sessionById = new Map<string, SessionPayload>();
  for (const session of sessions.data ?? []) sessionById.set(session.id, session);

  const tabsRef = React.useRef<HTMLDivElement | null>(null);
  const tabElements = React.useRef<Map<string, HTMLElement> | null>(null);
  const draggedRef = React.useRef(false);
  const [tabDrag, setTabDrag] = React.useState<OsDeckTabDrag | null>(null);
  const [pointerEpoch, setPointerEpoch] = React.useState(0);
  const pointerState = React.useRef<{
    windowId: string;
    startX: number;
    startY: number;
    active: boolean;
  } | null>(null);

  const registerTab = (windowId: string, element: HTMLElement | null) => {
    if (element === null) {
      tabElements.current?.delete(windowId);
      return;
    }
    const elements = tabElements.current ?? new Map<string, HTMLElement>();
    tabElements.current = elements;
    elements.set(windowId, element);
  };

  // The store owns the drop-target fact (the drag commit reads it); this deck
  // renders its own slot from that single source.
  const dropIndex = useSelector(windowManagerStore, snapshot =>
    snapshot.context.deckDropTarget?.frameId === frame.id
      ? snapshot.context.deckDropTarget.insertIndex
      : null
  );
  const foreignGestureActive = useSelector(
    windowManagerStore,
    snapshot =>
      snapshot.context.gesture?.status === "active" &&
      !frame.members.includes(snapshot.context.gesture.source.windowId)
  );
  const workAreaOrigin = useSelector(
    windowManagerStore,
    snapshot => snapshot.context.workArea?.origin ?? null,
    (previous, next) => previous?.x === next?.x && previous?.y === next?.y
  );
  // The pointer event is the source gesture boundary, so hit-testing and target
  // publication happen in the same event rather than in a render-following effect.
  React.useEffect(() => {
    if (!foreignGestureActive) {
      windowManagerStore.trigger.deckDropCleared({ frameId: frame.id });
      return;
    }
    const onPointerMove = (event: globalThis.PointerEvent) => {
      updateDeckDropTarget({
        frame,
        tabs: tabsRef.current,
        tabElements: tabElements.current ?? EMPTY_TAB_ELEMENTS,
        clientX: event.clientX,
        clientY: event.clientY,
      });
    };
    window.addEventListener("pointermove", onPointerMove);
    return () => {
      window.removeEventListener("pointermove", onPointerMove);
      windowManagerStore.trigger.deckDropCleared({ frameId: frame.id });
    };
  }, [foreignGestureActive, frame]);

  const clearTabDrag = () => {
    pointerState.current = null;
    setTabDrag(null);
  };

  React.useEffect(() => {
    if (pointerState.current === null) return;
    const onMove = (event: globalThis.PointerEvent) => {
      const current = pointerState.current;
      if (!current) return;
      if (!current.active) {
        if (
          Math.abs(event.clientX - current.startX) < DRAG_START_SLOP &&
          Math.abs(event.clientY - current.startY) < DRAG_START_SLOP
        ) {
          return;
        }
        current.active = true;
        draggedRef.current = true;
      }
      const state = manager.getState();
      const pinned = state.windows[current.windowId]?.pinned ?? false;
      const pinnedCount = frame.members.filter(member => state.windows[member]?.pinned).length;
      const tabs = tabsRef.current;
      const box = tabs?.getBoundingClientRect();
      const insideRow =
        box !== undefined &&
        event.clientY >= box.top - DECK_EXIT_SLOP &&
        event.clientY <= box.bottom + DECK_EXIT_SLOP;
      if (insideRow) autoScrollAtEdges(tabs, event.clientX);
      setTabDrag(
        insideRow
          ? {
              windowId: current.windowId,
              mode: "reorder",
              insertIndex: clampInsertToRegion(
                pinned,
                insertSlotAt(
                  frame.members,
                  tabElements.current ?? EMPTY_TAB_ELEMENTS,
                  event.clientX
                ),
                pinnedCount
              ),
            }
          : { windowId: current.windowId, mode: "out", insertIndex: null }
      );
    };
    const onUp = (event: globalThis.PointerEvent) => {
      const current = pointerState.current;
      clearTabDrag();
      if (!current?.active) return;
      const tabs = tabsRef.current;
      const box = tabs?.getBoundingClientRect();
      const insideRow =
        box !== undefined &&
        event.clientY >= box.top - DECK_EXIT_SLOP &&
        event.clientY <= box.bottom + DECK_EXIT_SLOP;
      if (insideRow) {
        const state = manager.getState();
        const pinned = state.windows[current.windowId]?.pinned ?? false;
        const pinnedCount = frame.members.filter(member => state.windows[member]?.pinned).length;
        const slot = clampInsertToRegion(
          pinned,
          insertSlotAt(frame.members, tabElements.current ?? EMPTY_TAB_ELEMENTS, event.clientX),
          pinnedCount
        );
        const currentIndex = frame.members.indexOf(current.windowId);
        const finalIndex = slot > currentIndex ? slot - 1 : slot;
        if (currentIndex >= 0 && finalIndex !== currentIndex) {
          manager.reorderStackMember(current.windowId, finalIndex);
        }
        return;
      }
      const layerX = event.clientX - (workAreaOrigin?.x ?? 0);
      const layerY = event.clientY - (workAreaOrigin?.y ?? 0);
      const rect: OsRect = {
        x: Math.round(layerX - frame.rect.w / 4),
        y: Math.round(layerY - 16),
        w: frame.rect.w,
        h: frame.rect.h,
      };
      const outcome = manager.toggleFloating(current.windowId, rect);
      if (outcome.accepted) {
        void outcome.completion.then(applied => {
          if (applied) void coordinator.userActivateWindow(current.windowId);
        });
      }
    };
    const onCancel = () => clearTabDrag();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") clearTabDrag();
    };
    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
    window.addEventListener("pointercancel", onCancel);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
      window.removeEventListener("pointercancel", onCancel);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [coordinator, frame, manager, pointerEpoch, workAreaOrigin]);

  const handleTabPointerDown = (windowId: string, event: React.PointerEvent<HTMLElement>) => {
    draggedRef.current = false;
    pointerState.current = {
      windowId,
      startX: event.clientX,
      startY: event.clientY,
      active: false,
    };
    setPointerEpoch(epoch => epoch + 1);
  };

  return {
    sessionById,
    tabDrag,
    dropIndex,
    otherWindowCount,
    tabsRef,
    registerTab,
    handleTabPointerDown,
    wasDragged: () => draggedRef.current,
    activateTab: windowId => {
      clearTabDrag();
      cancelActiveWindowGesture();
      void coordinator.userFocus(windowId);
    },
    closeTab: windowId => {
      clearTabDrag();
      cancelActiveWindowGesture();
      void coordinator.userClose(windowId);
    },
    closeOthers: windowId => {
      void manager.closeWindowScoped(windowId, "others");
    },
    closeRight: windowId => {
      void manager.closeWindowScoped(windowId, "right");
    },
    pinTab: (windowId, pinned) => {
      manager.pinWindow(windowId, pinned);
    },
    moveToNewWindow: windowId => {
      const outcome = manager.toggleFloating(windowId);
      if (outcome.accepted) {
        void outcome.completion.then(applied => {
          if (applied) void coordinator.userActivateWindow(windowId);
        });
      }
    },
    mergeAllWindows: () => {
      const state = manager.getState();
      const joiners = Object.values(state.windows)
        .filter(win => win.desktopId === frame.desktopId && !memberIds.has(win.id))
        .sort((left, right) => left.id.localeCompare(right.id))
        .map(win => win.id);
      if (joiners.length === 0) return;
      manager.groupWindows(frame.activeWindowId, joiners);
    },
    openNewTab: () => {
      void coordinator
        .userOpen({ app: "new-tab", stackTargetWindowId: frame.activeWindowId })
        .then(windowId => {
          if (windowId !== null) {
            windowManagerStore.trigger.paletteIntentRequested({
              intent: { kind: "destination", windowId },
            });
          }
        });
    },
  };
}
