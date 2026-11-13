import { useSelector } from "@xstate/store-react";
import type { PointerEvent as ReactPointerEvent } from "react";

import { contentDropAt, dropFits, TAB_DRAG_EXIT_PX, type ContentDrop } from "../lib/drop-edge";
import type { OsDesktopRuntimeStore, WindowManagerController } from "../lib/os-types";
import { leadWindowId } from "../lib/tab-model";
import { buildWindowManagerMinimums } from "../lib/window-manager-view";
import type {
  LayoutNode,
  PixelPoint,
  TabId,
  WindowManagerDragModifier,
  WindowId,
} from "../lib/window-manager-types";
import {
  osTabDragStore,
  type TabDragIntent,
  type TabDragSession,
  type TabDragSource,
  type TabDragZone,
} from "../stores/os-tab-drag-store";
import { useOsShell } from "./use-os-shell";

const INCOMING_LEAF_ID = "drop-preview-leaf";

function tabSource(state: OsDesktopRuntimeStore, tabId: TabId): TabDragSource | null {
  for (const desktop of state.desktops) {
    const tab = desktop.tabs.find(candidate => candidate.id === tabId);
    if (tab === undefined) continue;
    const windowId = leadWindowId(tab, state.client?.focusOrder ?? []);
    const nodeId = windowId === null ? null : state.windows[windowId]?.nodeId;
    if (windowId === null || !nodeId) return null;
    return { kind: "tab", tabId, windowId, nodeId };
  }
  return null;
}

/** A head drags one member of a split; the head of a single-window tab drags its tab. */
function headSource(state: OsDesktopRuntimeStore, windowId: WindowId): TabDragSource | null {
  const win = state.windows[windowId];
  if (!win?.nodeId) return null;
  if (!win.inSplit) return tabSource(state, win.tabId);
  return { kind: "head", tabId: win.tabId, windowId, nodeId: win.nodeId };
}

/**
 * The tab a dragged tab splits into: the active tab, or — when the dragged tab is the active one —
 * the tab of the window this client focused before it on the same desktop, else a neighbor.
 */
function splitTabFor(
  state: OsDesktopRuntimeStore,
  tabOrder: readonly TabId[],
  sourceTabId: TabId,
  activeTabId: TabId | null
): TabId | null {
  if (activeTabId !== null && activeTabId !== sourceTabId) return activeTabId;
  for (const windowId of state.client?.focusOrder ?? []) {
    const tabId = state.windows[windowId]?.tabId;
    if (tabId !== undefined && tabId !== sourceTabId && tabOrder.includes(tabId)) return tabId;
  }
  const index = tabOrder.indexOf(sourceTabId);
  return tabOrder[index + 1] ?? tabOrder[index - 1] ?? null;
}

function dragSession(
  state: OsDesktopRuntimeStore,
  source: TabDragSource | null
): TabDragSession | null {
  const revision = state.snapshot?.revision;
  if (source === null || revision === undefined) return null;
  const desktop = state.desktops.find(candidate =>
    candidate.tabs.some(tab => tab.id === source.tabId)
  );
  if (desktop === undefined || desktop.id !== state.activeDesktopId) return null;
  const tabOrder = desktop.tabs.map(tab => tab.id);
  const activeTabId = state.activeTabIds[desktop.id] ?? null;
  return {
    desktopId: desktop.id,
    revision,
    source,
    tabOrder,
    pinnedCount: desktop.tabs.filter(tab => tab.pinned).length,
    activeTabId,
    splitTabId: splitTabFor(state, tabOrder, source.tabId, activeTabId),
  };
}

function modifierHeld(
  modifier: WindowManagerDragModifier | undefined,
  event: KeyboardEvent | PointerEvent
) {
  switch (modifier) {
    case "shift":
      return event.shiftKey;
    case "alt":
      return event.altKey;
    case "control":
      return event.ctrlKey;
    case "meta":
      return event.metaKey;
    default:
      return false;
  }
}

function within(rect: DOMRect, point: PixelPoint): boolean {
  return (
    point.x >= rect.left && point.x <= rect.right && point.y >= rect.top && point.y <= rect.bottom
  );
}

/** The bar position under the pointer: the insertion index among every tab and the tab under it. */
function barZone(bar: Element, point: PixelPoint): TabDragZone {
  const tabs = Array.from(bar.querySelectorAll<HTMLElement>('[data-slot="os-tab"]'));
  let index = tabs.findIndex(tab => {
    const rect = tab.getBoundingClientRect();
    return point.x < rect.left + rect.width / 2;
  });
  if (index < 0) index = tabs.length;
  const over = tabs.find(tab => {
    const rect = tab.getBoundingClientRect();
    return point.x >= rect.left && point.x <= rect.right;
  });
  return { kind: "bar", index, overTabId: over?.dataset.tabId ?? null };
}

function incomingNode(state: OsDesktopRuntimeStore, source: TabDragSource): LayoutNode | null {
  if (source.kind === "head")
    return { id: INCOMING_LEAF_ID, kind: "leaf", windowId: source.windowId };
  for (const desktop of state.desktops) {
    const tab = desktop.tabs.find(candidate => candidate.id === source.tabId);
    if (tab !== undefined) return tab.root;
  }
  return null;
}

/** The drop over the shown tab's content, withheld where a member would go below its minimum. */
function contentDrop(
  state: OsDesktopRuntimeStore,
  session: TabDragSession,
  contentTabId: TabId | null,
  point: PixelPoint,
  swapHeld: boolean
): ContentDrop | null {
  const projection = contentTabId === null ? undefined : state.projections[contentTabId];
  const tab = state.desktops
    .find(desktop => desktop.id === session.desktopId)
    ?.tabs.find(candidate => candidate.id === contentTabId);
  const config = state.windowManagerConfig;
  if (projection === undefined || tab === undefined || config === null) return null;
  const drop = contentDropAt(projection, point);
  if (drop === null) return null;
  const { source } = session;
  const sameTab = source.kind === "head" && source.tabId === tab.id;
  // A swap keeps both shares, so the minimum rule does not apply to it.
  if (swapHeld && sameTab && drop.kind === "window") return drop;
  const incoming = incomingNode(state, source);
  if (incoming === null || state.snapshot === null) return null;
  const fits = dropFits({
    tab,
    incoming,
    ...(sameTab ? { leavingWindowId: source.windowId } : {}),
    drop,
    workArea: projection.workArea,
    gaps: config.gaps,
    minimums: buildWindowManagerMinimums(state.snapshot),
  });
  return fits ? drop : null;
}

function resolveZone(
  state: OsDesktopRuntimeStore,
  session: TabDragSession,
  point: PixelPoint,
  swapHeld: boolean
): TabDragZone {
  const bar = document.querySelector('[data-slot="os-tab-bar"]');
  if (bar !== null) {
    const rect = bar.getBoundingClientRect();
    const onBar =
      point.x >= rect.left &&
      point.x <= rect.right &&
      point.y >= rect.top - TAB_DRAG_EXIT_PX &&
      point.y <= rect.bottom + TAB_DRAG_EXIT_PX;
    if (onBar) return barZone(bar, point);
  }
  const layer = document.querySelector('[data-slot="os-desk-layer"]');
  if (layer === null) return { kind: "outside" };
  const rect = layer.getBoundingClientRect();
  if (!within(rect, point)) return { kind: "outside" };
  const { source } = session;
  const contentTabId =
    source.kind === "tab"
      ? session.splitTabId
      : (osTabDragStore.getSnapshot().context.contentTabId ?? source.tabId);
  const local = { x: point.x - rect.left, y: point.y - rect.top };
  return { kind: "content", drop: contentDrop(state, session, contentTabId, local, swapHeld) };
}

function dispatchIntent(manager: WindowManagerController, intent: TabDragIntent): void {
  switch (intent.kind) {
    case "reorder":
      void manager
        .reorderTab(intent.windowId, intent.index, intent.fence)
        .completion.finally(() => osTabDragStore.trigger.settled());
      return;
    case "move":
      manager.moveWindow(intent.windowId, intent.input, intent.fence);
      return;
    case "swap":
      manager.swapWindows(intent.firstWindowId, intent.secondWindowId, intent.fence);
      return;
  }
}

/** The click that ends a drag must not activate the tab or press the control under it. */
function swallowNextClick(): void {
  const swallow = (event: MouseEvent) => {
    event.preventDefault();
    event.stopPropagation();
  };
  window.addEventListener("click", swallow, { capture: true, once: true });
  setTimeout(() => window.removeEventListener("click", swallow, { capture: true }), 0);
}

/** One pointer gesture from press to release: pointer, keys and cancellation feed the store. */
function trackGesture(
  manager: WindowManagerController,
  session: TabDragSession,
  event: ReactPointerEvent<HTMLElement>
): void {
  const element = event.currentTarget;
  const pointerId = event.pointerId;
  let point: PixelPoint = { x: event.clientX, y: event.clientY };
  let swapHeld = false;
  const swapModifier = () => manager.getState().windowManagerConfig?.swapModifier;
  const dropped = osTabDragStore.on("dropped", ({ intent }) => dispatchIntent(manager, intent));
  const root = document.documentElement;
  const previousUserSelect = root.style.userSelect;

  const update = () => {
    const state = manager.getState();
    osTabDragStore.trigger.moved({
      point,
      zone: resolveZone(state, session, point, swapHeld),
      swapHeld,
    });
    if (osTabDragStore.getSnapshot().context.phase === "dragging") root.style.userSelect = "none";
  };
  const stop = () => {
    window.removeEventListener("pointermove", onMove, true);
    window.removeEventListener("pointerup", onUp, true);
    window.removeEventListener("pointercancel", onCancel, true);
    window.removeEventListener("keydown", onKey, true);
    window.removeEventListener("keyup", onKey, true);
    window.removeEventListener("blur", onCancel);
    dropped.unsubscribe();
    root.style.userSelect = previousUserSelect;
    if (element.hasPointerCapture(pointerId)) element.releasePointerCapture(pointerId);
  };
  const onMove = (move: PointerEvent) => {
    if (move.pointerId !== pointerId) return;
    point = { x: move.clientX, y: move.clientY };
    swapHeld = modifierHeld(swapModifier(), move);
    update();
  };
  const onUp = (up: PointerEvent) => {
    if (up.pointerId !== pointerId) return;
    point = { x: up.clientX, y: up.clientY };
    swapHeld = modifierHeld(swapModifier(), up);
    update();
    const dragged = osTabDragStore.getSnapshot().context.phase === "dragging";
    stop();
    osTabDragStore.trigger.released();
    if (dragged) swallowNextClick();
  };
  const onCancel = () => {
    stop();
    osTabDragStore.trigger.cancelled();
  };
  const onKey = (key: KeyboardEvent) => {
    if (key.type === "keydown" && key.key === "Escape") {
      if (osTabDragStore.getSnapshot().context.phase === "dragging") {
        key.preventDefault();
        key.stopPropagation();
      }
      onCancel();
      return;
    }
    const held = modifierHeld(swapModifier(), key);
    if (held === swapHeld) return;
    swapHeld = held;
    if (osTabDragStore.getSnapshot().context.phase === "dragging") update();
  };

  try {
    element.setPointerCapture(pointerId);
  } catch {
    // A pointer that already ended cannot be captured; window listeners still see the release.
  }
  window.addEventListener("pointermove", onMove, true);
  window.addEventListener("pointerup", onUp, true);
  window.addEventListener("pointercancel", onCancel, true);
  window.addEventListener("keydown", onKey, true);
  window.addEventListener("keyup", onKey, true);
  window.addEventListener("blur", onCancel);
  osTabDragStore.trigger.pressed({ session, point });
}

/**
 * Starts tab and window-head drags. Nothing reaches the daemon until the drop: the bar order and
 * the shown tab are local previews, and the drop sends one command fenced to the layout the drag
 * started from.
 */
export function useOsTabDragStart() {
  const { manager } = useOsShell();
  const start = (
    event: ReactPointerEvent<HTMLElement>,
    pick: (state: OsDesktopRuntimeStore) => TabDragSource | null
  ) => {
    if (event.button !== 0 || osTabDragStore.getSnapshot().context.phase !== "idle") return;
    const origin = event.target instanceof Element ? event.target : null;
    if (origin?.closest("input, textarea, select, [contenteditable='true']")) return;
    const state = manager.getState();
    const session = dragSession(state, pick(state));
    if (session !== null) trackGesture(manager, session, event);
  };
  return {
    startTabDrag: (tabId: TabId, event: ReactPointerEvent<HTMLElement>) =>
      start(event, state => tabSource(state, tabId)),
    startHeadDrag: (windowId: WindowId, event: ReactPointerEvent<HTMLElement>) =>
      start(event, state => headSource(state, windowId)),
  };
}

/** The drag's view for the bar, the content and the ghost. */
export function useOsTabDrag() {
  return useSelector(osTabDragStore, snapshot => snapshot.context);
}

/** Whether this window's head is the one being dragged; read per pane, so it stays cheap. */
export function useOsWindowDragged(windowId: WindowId): boolean {
  return useSelector(
    osTabDragStore,
    snapshot =>
      snapshot.context.phase === "dragging" &&
      snapshot.context.session?.source.kind === "head" &&
      snapshot.context.session.source.windowId === windowId
  );
}

/** Whether a drag shows this tab's content as a local preview while another tab stays active. */
export function useOsTabPreviewed(tabId: TabId | null): boolean {
  return useSelector(
    osTabDragStore,
    snapshot =>
      tabId !== null &&
      snapshot.context.phase === "dragging" &&
      snapshot.context.contentTabId === tabId &&
      snapshot.context.session?.activeTabId !== tabId
  );
}
