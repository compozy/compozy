import { useSelector, useStore } from "@xstate/store-react";
import * as React from "react";

import type { OsWindowFrameModel } from "../lib/group-projection";
import type { OsRect } from "../lib/os-types";
import { autoScrollDeckAtEdges, insertSlotAt, pointInsideDeck } from "../lib/window-deck-geometry";
import { registerWindowDeckMergeTarget } from "../lib/window-merge-target-coordinator";
import { windowManagerStore } from "../stores/window-manager-store";
import {
  osWindowDeckDragLogic,
  type OsDeckTabDrag,
  type OsWindowDeckDragRuntime,
} from "./os-window-deck-drag-store";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";
import { type SessionPayload, useSessions } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

const DECK_EXIT_SLOP = 28;

export type { OsDeckTabDrag } from "./os-window-deck-drag-store";

export interface OsWindowDeckModel {
  sessionById: ReadonlyMap<string, SessionPayload>;
  tabDrag: OsDeckTabDrag | null;
  /** Insertion slot advertised to the active window drag over this deck. */
  dropIndex: number | null;
  otherWindowCount: number;
  registerTabs: (element: HTMLDivElement | null) => void;
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

function cancelActiveWindowGesture(): void {
  const gesture = windowManagerStore.getSnapshot().context.gesture;
  if (gesture?.status === "active") {
    windowManagerStore.trigger.gestureCancelled({ reason: "manual" });
  }
}

/**
 * Deck view-model: shared session data, tab commands, and one event-driven tab
 * gesture store. Browser listeners and animation frames exist only for an
 * active press; the shared merge coordinator handles foreign frame drags.
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
  const [tabElements] = React.useState(() => new Map<string, HTMLElement>());
  const dragStore = useStore(osWindowDeckDragLogic);
  const tabDrag = useSelector(dragStore, snapshot => snapshot.context.preview);
  const dropIndex = useSelector(windowManagerStore, snapshot =>
    snapshot.context.deckDropTarget?.frameId === frame.id
      ? snapshot.context.deckDropTarget.insertIndex
      : null
  );
  const readCommittedFrame = React.useEffectEvent(() => frame);

  React.useEffect(() => () => dragStore.trigger.disposed(), [dragStore]);

  const frameId = frame.id;
  React.useEffect(
    () =>
      registerWindowDeckMergeTarget(manager, {
        frameId,
        getFrame: readCommittedFrame,
        getTabElements: () => tabElements,
        getTabs: () => tabsRef.current,
      }),
    [frameId, manager, tabElements]
  );

  const registerTab = (windowId: string, element: HTMLElement | null) => {
    if (element === null) tabElements.delete(windowId);
    else tabElements.set(windowId, element);
  };

  const currentFrame = (): OsWindowFrameModel =>
    manager.getState().frames[frame.desktopId]?.find(candidate => candidate.id === frame.id) ??
    frame;

  const activateAfterAcceptedCommand = (windowId: string, completion: Promise<boolean>) => {
    void completion
      .then(applied => {
        if (applied) return coordinator.userActivateWindow(windowId);
      })
      .catch(error => console.error("Failed to activate a moved window", error));
  };

  const dragRuntime = (): OsWindowDeckDragRuntime => ({
    project: (windowId, point) => {
      const projectedFrame = currentFrame();
      const state = manager.getState();
      const pinned = state.windows[windowId]?.pinned ?? false;
      const pinnedCount = projectedFrame.members.filter(
        member => state.windows[member]?.pinned
      ).length;
      const tabs = tabsRef.current;
      if (tabs === null) return { windowId, mode: "out", insertIndex: null };
      const bounds = tabs.getBoundingClientRect();
      const insideRow = pointInsideDeck(
        tabs,
        point,
        { top: DECK_EXIT_SLOP, bottom: DECK_EXIT_SLOP },
        bounds
      );
      if (!insideRow) return { windowId, mode: "out", insertIndex: null };
      autoScrollDeckAtEdges(tabs, point.clientX, bounds);
      return {
        windowId,
        mode: "reorder",
        insertIndex: clampInsertToRegion(
          pinned,
          insertSlotAt(projectedFrame.members, tabElements, point.clientX),
          pinnedCount
        ),
      };
    },
    commit: (windowId, point) => {
      const projectedFrame = currentFrame();
      const tabs = tabsRef.current;
      const bounds = tabs?.getBoundingClientRect();
      const insideRow =
        tabs !== null &&
        bounds !== undefined &&
        pointInsideDeck(tabs, point, { top: DECK_EXIT_SLOP, bottom: DECK_EXIT_SLOP }, bounds);
      if (insideRow) {
        const state = manager.getState();
        const pinned = state.windows[windowId]?.pinned ?? false;
        const pinnedCount = projectedFrame.members.filter(
          member => state.windows[member]?.pinned
        ).length;
        const slot = clampInsertToRegion(
          pinned,
          insertSlotAt(projectedFrame.members, tabElements, point.clientX),
          pinnedCount
        );
        const currentIndex = projectedFrame.members.indexOf(windowId);
        const finalIndex = slot > currentIndex ? slot - 1 : slot;
        if (currentIndex >= 0 && finalIndex !== currentIndex) {
          manager.reorderStackMember(windowId, finalIndex);
        }
        return;
      }
      const origin = windowManagerStore.getSnapshot().context.workArea?.origin ?? { x: 0, y: 0 };
      const rect: OsRect = {
        x: Math.round(point.clientX - origin.x - projectedFrame.rect.w / 4),
        y: Math.round(point.clientY - origin.y - 16),
        w: projectedFrame.rect.w,
        h: projectedFrame.rect.h,
      };
      const outcome = manager.toggleFloating(windowId, rect);
      if (outcome.accepted) activateAfterAcceptedCommand(windowId, outcome.completion);
    },
  });

  const clearTabDrag = () => {
    const generation = dragStore.getSnapshot().context.generation;
    dragStore.trigger.pointerCancelled({ generation });
  };

  return {
    sessionById,
    tabDrag,
    dropIndex,
    otherWindowCount,
    registerTabs: element => {
      tabsRef.current = element;
    },
    registerTab,
    handleTabPointerDown: (windowId, event) => {
      dragStore.trigger.pointerPressed({
        point: { clientX: event.clientX, clientY: event.clientY },
        runtime: dragRuntime(),
        windowId,
      });
    },
    wasDragged: () => dragStore.getSnapshot().context.dragged,
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
      if (outcome.accepted) activateAfterAcceptedCommand(windowId, outcome.completion);
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
        })
        .catch(error => console.error("Failed to open a new window tab", error));
    },
  };
}
