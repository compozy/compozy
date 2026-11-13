import { createStore } from "@xstate/store";

import { HEAD_DWELL_MS, type ContentDrop, type DropEdge } from "../lib/drop-edge";
import type { MoveWindowInput, WindowManagerCommandFence } from "../lib/os-types";
import type {
  DesktopId,
  LayoutNodeId,
  LayoutRevision,
  PixelPoint,
  TabId,
  WindowId,
} from "../lib/window-manager-types";

/** Pointer travel before a press becomes a drag; below it a press stays a click. */
export const DRAG_START_PX = 5;
/** Pointer travel that restarts a head's dwell over a tab. */
const DWELL_SLOP_PX = 4;

/**
 * What is dragged. A tab carries its whole tree; a window head carries one member of a split.
 * The head of a single-window tab is dragged as its tab.
 */
export type TabDragSource =
  | { kind: "tab"; tabId: TabId; windowId: WindowId; nodeId: LayoutNodeId }
  | { kind: "head"; tabId: TabId; windowId: WindowId; nodeId: LayoutNodeId };

/** Bar facts captured when the drag starts; the drag never reads the live bar again. */
export interface TabDragSession {
  desktopId: DesktopId;
  /** Layout revision the drag was authored against: the drop is fenced to it. */
  revision: LayoutRevision;
  source: TabDragSource;
  /** The bar's tab order at press time. */
  tabOrder: readonly TabId[];
  /** The lead window of each tab in `tabOrder`: what an unsplit drop names as its anchor. */
  tabLeads: readonly WindowId[];
  pinnedCount: number;
  /** The tab this client showed before the drag; Escape returns to it. */
  activeTabId: TabId | null;
  /**
   * The tab a dragged tab splits into once it leaves the bar: the active tab, or the previously
   * active one when the dragged tab is the active tab. Null when there is no other tab.
   */
  splitTabId: TabId | null;
}

/**
 * Where the pointer is, resolved by the shell's hit test: the bar, the shown tab's content, or a
 * desktop drop target (an overview card, a top-bar indicator). Each is a variant; the gesture
 * rules stay here.
 */
export type TabDragZone =
  | { kind: "bar"; index: number; overTabId: TabId | null }
  | { kind: "content"; drop: ContentDrop | null }
  | { kind: "desktop"; desktopId: DesktopId }
  | { kind: "outside" };

export type TabDragTarget =
  | { kind: "reorder"; index: number }
  /** A head dropped on the bar becomes its own tab before the tab at `index` (US-010.AC-2). */
  | { kind: "unsplit"; index: number }
  | { kind: "window"; tabId: TabId; windowId: WindowId; nodeId: LayoutNodeId; edge: DropEdge }
  | { kind: "root"; tabId: TabId; edge: DropEdge }
  | { kind: "swap"; windowId: WindowId; nodeId: LayoutNodeId }
  /** The dragged tab (or head, as its own tab) joins the end of another desktop's bar (US-016). */
  | { kind: "desktop"; desktopId: DesktopId };

/** The one command a finished drag sends. */
export type TabDragIntent =
  | { kind: "reorder"; windowId: WindowId; index: number; fence: WindowManagerCommandFence }
  | { kind: "move"; windowId: WindowId; input: MoveWindowInput; fence: WindowManagerCommandFence }
  /**
   * `window.move` with placement `tab`, anchored on the tab before the marker; `reorderIndex` is
   * the one follow-up reorder for a marker no tab precedes (the bar's first position).
   */
  | {
      kind: "unsplit";
      windowId: WindowId;
      input: MoveWindowInput;
      reorderIndex: number | null;
      fence: WindowManagerCommandFence;
    }
  | {
      kind: "swap";
      firstWindowId: WindowId;
      secondWindowId: WindowId;
      fence: WindowManagerCommandFence;
    };

export interface TabDragState {
  phase: "idle" | "pressed" | "dragging";
  session: TabDragSession | null;
  origin: PixelPoint | null;
  pointer: PixelPoint | null;
  /** Reorder while the pointer is on the bar, split once it is in the content. */
  mode: "reorder" | "split" | null;
  /** The tab whose content shows during the drag: a local preview, never a command. */
  contentTabId: TabId | null;
  /** Bar index (among every tab) before which the insertion marker shows. */
  insertionIndex: number | null;
  target: TabDragTarget | null;
  /** The tab a dragged head is held over, and where the hold started. */
  dwell: { tabId: TabId; anchor: PixelPoint } | null;
  /** The bar order a reorder drop committed, held until the daemon answers. */
  settling: { desktopId: DesktopId; order: readonly TabId[] } | null;
}

function idle(settling: TabDragState["settling"]): TabDragState {
  return {
    phase: "idle",
    session: null,
    origin: null,
    pointer: null,
    mode: null,
    contentTabId: null,
    insertionIndex: null,
    target: null,
    dwell: null,
    settling,
  };
}

function distance(a: PixelPoint, b: PixelPoint): number {
  return Math.hypot(a.x - b.x, a.y - b.y);
}

/** An unsplit window becomes an unpinned tab (US-010.EC-3): its marker stays after the pins. */
function unpinnedInsertion(session: TabDragSession, index: number): number {
  return Math.max(session.pinnedCount, Math.min(index, session.tabOrder.length));
}

/**
 * The one move an unsplit drop sends. The daemon places the new tab right after the target
 * window's tab, or right after its source tab with no target, so the tab before the marker is the
 * anchor; only the bar's first position has no anchor and takes a follow-up reorder.
 */
export function unsplitIntent(
  session: TabDragSession,
  insertionIndex: number
): Extract<TabDragIntent, { kind: "unsplit" }> {
  const { source } = session;
  const sourceIndex = session.tabOrder.indexOf(source.tabId);
  const anchorIndex = insertionIndex - 1;
  const anchorTabId = session.tabOrder[anchorIndex];
  const anchorLead = session.tabLeads[anchorIndex];
  const afterSource = anchorIndex === sourceIndex || anchorTabId === undefined;
  return {
    kind: "unsplit",
    windowId: source.windowId,
    input: {
      destinationDesktopId: session.desktopId,
      placement: "tab",
      ...(afterSource || anchorLead === undefined ? {} : { targetWindowId: anchorLead }),
    },
    reorderIndex: anchorTabId === undefined ? insertionIndex : null,
    fence: {
      expectedRevision: session.revision,
      rebase: { windowId: source.windowId, sourceNodeId: source.nodeId },
    },
  };
}

/** The insertion index clamped to the source's range: pinned tabs among pinned, others after. */
function clampedInsertion(session: TabDragSession, index: number): number {
  const sourceIndex = session.tabOrder.indexOf(session.source.tabId);
  const sourcePinned = sourceIndex >= 0 && sourceIndex < session.pinnedCount;
  if (sourcePinned) return Math.max(0, Math.min(index, session.pinnedCount));
  return Math.max(session.pinnedCount, Math.min(index, session.tabOrder.length));
}

/** The bar order after moving the source before `insertionIndex`. */
export function reorderedTabs(
  order: readonly TabId[],
  sourceTabId: TabId,
  insertionIndex: number
): TabId[] {
  const before = order.slice(0, insertionIndex).filter(tabId => tabId !== sourceTabId);
  const after = order.slice(insertionIndex).filter(tabId => tabId !== sourceTabId);
  return [...before, sourceTabId, ...after];
}

function contentTarget(
  drop: ContentDrop | null,
  tabId: TabId | null,
  source: TabDragSource,
  swapHeld: boolean
): TabDragTarget | null {
  if (drop === null || tabId === null) return null;
  if (drop.kind === "root") return { kind: "root", tabId, edge: drop.edge };
  // Dropping a window on itself changes nothing (US-008.EC-1).
  if (drop.windowId === source.windowId) return null;
  // The swap modifier swaps two windows of one tab; across tabs it is ignored (US-008.EC-2).
  if (swapHeld && source.kind === "head" && tabId === source.tabId) {
    return { kind: "swap", windowId: drop.windowId, nodeId: drop.nodeId };
  }
  return { kind: "window", tabId, ...drop };
}

function fence(
  session: TabDragSession,
  target: Exclude<TabDragTarget, { kind: "unsplit" }>
): WindowManagerCommandFence {
  const { windowId, nodeId } = session.source;
  const rebase =
    target.kind === "reorder"
      ? { windowId }
      : target.kind === "root"
        ? { windowId, sourceNodeId: nodeId, tabId: target.tabId }
        : target.kind === "desktop"
          ? { windowId, sourceNodeId: nodeId }
          : { windowId, sourceNodeId: nodeId, targetNodeId: target.nodeId };
  return { expectedRevision: session.revision, rebase };
}

/** The command a drop means, or null when the drop changes nothing. */
export function tabDragIntent(state: TabDragState): TabDragIntent | null {
  const { session, target } = state;
  if (state.phase !== "dragging" || session === null || target === null) return null;
  const { source } = session;
  if (target.kind === "reorder") {
    const current = session.tabOrder.indexOf(source.tabId);
    if (current === target.index) return null;
    return {
      kind: "reorder",
      windowId: source.windowId,
      index: target.index,
      fence: fence(session, target),
    };
  }
  if (target.kind === "unsplit") return unsplitIntent(session, target.index);
  if (target.kind === "swap") {
    return {
      kind: "swap",
      firstWindowId: source.windowId,
      secondWindowId: target.windowId,
      fence: fence(session, target),
    };
  }
  if (target.kind === "desktop") {
    return {
      kind: "move",
      windowId: source.windowId,
      input: {
        destinationDesktopId: target.desktopId,
        placement: "tab",
        moveGroup: source.kind === "tab",
      },
      fence: fence(session, target),
    };
  }
  const input: MoveWindowInput = {
    destinationDesktopId: session.desktopId,
    placement: target.edge,
    moveGroup: source.kind === "tab",
    ...(target.kind === "window"
      ? { targetWindowId: target.windowId }
      : { targetTabId: target.tabId }),
  };
  return { kind: "move", windowId: source.windowId, input, fence: fence(session, target) };
}

function onBar(
  state: TabDragState,
  zone: Extract<TabDragZone, { kind: "bar" }>,
  point: PixelPoint
): TabDragState {
  const session = state.session!;
  if (session.source.kind === "tab") {
    const insertionIndex = clampedInsertion(session, zone.index);
    const order = reorderedTabs(session.tabOrder, session.source.tabId, insertionIndex);
    return {
      ...state,
      mode: "reorder",
      contentTabId: session.activeTabId,
      insertionIndex,
      target: { kind: "reorder", index: order.indexOf(session.source.tabId) },
      dwell: null,
    };
  }
  // A head over the bar shows an insertion marker (drop = unsplit there) and dwells on the tab
  // under it: held still, that tab's content becomes the drop target (US-008.AC-3).
  const insertionIndex = unpinnedInsertion(session, zone.index);
  const over = zone.overTabId;
  let dwell: TabDragState["dwell"] = null;
  if (over !== null && over !== state.contentTabId) {
    const held =
      state.dwell?.tabId === over && distance(state.dwell.anchor, point) <= DWELL_SLOP_PX;
    dwell = held ? state.dwell : { tabId: over, anchor: point };
  }
  return {
    ...state,
    mode: "reorder",
    insertionIndex,
    target: { kind: "unsplit", index: insertionIndex },
    dwell,
  };
}

function onContent(
  state: TabDragState,
  zone: Extract<TabDragZone, { kind: "content" }>,
  swapHeld: boolean
): TabDragState {
  const session = state.session!;
  const contentTabId =
    session.source.kind === "tab"
      ? session.splitTabId
      : (state.contentTabId ?? session.source.tabId);
  return {
    ...state,
    mode: "split",
    contentTabId,
    insertionIndex: null,
    target: contentTarget(zone.drop, contentTabId, session.source, swapHeld),
    dwell: null,
  };
}

/** Over a desktop target: no preview; dropping on the drag's own desktop changes nothing. */
function onDesktop(
  state: TabDragState,
  zone: Extract<TabDragZone, { kind: "desktop" }>
): TabDragState {
  const session = state.session!;
  return {
    ...state,
    mode: null,
    contentTabId: session.source.kind === "head" ? session.source.tabId : session.activeTabId,
    insertionIndex: null,
    target:
      zone.desktopId === session.desktopId ? null : { kind: "desktop", desktopId: zone.desktopId },
    dwell: null,
  };
}

export type TabDragEvents = {
  pressed: { session: TabDragSession; point: PixelPoint };
  moved: { point: PixelPoint; zone: TabDragZone; swapHeld: boolean };
  dwellElapsed: { tabId: TabId };
  released: {};
  cancelled: {};
  settled: {};
};

export type TabDragEmitted = {
  dropped: { intent: TabDragIntent };
};

export function createOsTabDragStore() {
  let dwellTimer: ReturnType<typeof setTimeout> | null = null;
  const clearDwell = () => {
    if (dwellTimer !== null) clearTimeout(dwellTimer);
    dwellTimer = null;
  };
  // Bound once the store exists: the dwell timer reports back through the store's own event.
  let dwellElapsed: (tabId: TabId) => void = () => {};

  const store = createStore<TabDragState, TabDragEvents, TabDragEmitted>({
    context: idle(null),
    on: {
      pressed: (state, event: { session: TabDragSession; point: PixelPoint }) => {
        if (state.phase !== "idle") return;
        return {
          ...idle(state.settling),
          phase: "pressed" as const,
          session: event.session,
          origin: event.point,
          pointer: event.point,
          contentTabId:
            event.session.source.kind === "head"
              ? event.session.source.tabId
              : event.session.activeTabId,
        };
      },
      moved: (
        state,
        event: { point: PixelPoint; zone: TabDragZone; swapHeld: boolean },
        enqueue
      ) => {
        if (state.phase === "idle" || state.origin === null || state.session === null) return;
        if (state.phase === "pressed" && distance(state.origin, event.point) < DRAG_START_PX) {
          return;
        }
        const moving: TabDragState = { ...state, phase: "dragging", pointer: event.point };
        let next: TabDragState;
        if (event.zone.kind === "bar") next = onBar(moving, event.zone, event.point);
        else if (event.zone.kind === "content")
          next = onContent(moving, event.zone, event.swapHeld);
        else if (event.zone.kind === "desktop") next = onDesktop(moving, event.zone);
        else next = { ...moving, mode: null, insertionIndex: null, target: null, dwell: null };
        if (next.dwell !== state.dwell) {
          const dwell = next.dwell;
          enqueue.effect(() => {
            clearDwell();
            if (dwell === null) return;
            dwellTimer = setTimeout(() => {
              dwellTimer = null;
              dwellElapsed(dwell.tabId);
            }, HEAD_DWELL_MS);
          });
        }
        return next;
      },
      /** A head held still over a tab for the dwell time previews that tab's content. */
      dwellElapsed: (state, event: { tabId: TabId }) => {
        if (state.phase !== "dragging" || state.dwell?.tabId !== event.tabId) return;
        return { ...state, contentTabId: event.tabId, dwell: null };
      },
      released: (state, _event, enqueue) => {
        enqueue.effect(clearDwell);
        if (state.phase === "idle") return;
        const intent = tabDragIntent(state);
        if (intent === null) return idle(state.settling);
        enqueue.emit.dropped({ intent });
        const session = state.session!;
        const settling =
          intent.kind === "reorder" && state.insertionIndex !== null
            ? {
                desktopId: session.desktopId,
                order: reorderedTabs(session.tabOrder, session.source.tabId, state.insertionIndex),
              }
            : state.settling;
        return idle(settling);
      },
      /** Escape, a lost pointer or an unmounted source: nothing changes and nothing is sent. */
      cancelled: (state, _event, enqueue) => {
        enqueue.effect(clearDwell);
        if (state.phase === "idle") return;
        return idle(state.settling);
      },
      /** The daemon answered the reorder: the bar shows its order again. */
      settled: state => (state.settling === null ? undefined : { ...state, settling: null }),
    },
  });
  dwellElapsed = tabId => store.trigger.dwellElapsed({ tabId });
  return store;
}

export type OsTabDragStore = ReturnType<typeof createOsTabDragStore>;

/** The shell's one tab and window-head drag. */
export const osTabDragStore = createOsTabDragStore();
