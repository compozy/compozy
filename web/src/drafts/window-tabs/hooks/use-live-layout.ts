import { useRef, useState } from "react";

import type { DragGhostSource } from "../../window-tabs-shared/drag-ghost";
import { TAB } from "../../window-tabs-shared/fixtures";
import { leaf, leafIds, type DraftTab } from "../../window-tabs-shared/model";
import { resolveDrop } from "../live-drag";
import {
  applyDrop,
  closeTab,
  closeWindow,
  resizePair,
  tabOf,
  type Drag,
  type Layout,
} from "../live-layout";

const START: readonly DraftTab[] = [TAB.dash, TAB.auth, TAB.tabs, TAB.review, TAB.term, TAB.files];
const SPARE = ["docs", "flaky", "term-tests", "agents", "auto"];
const MIN_SHARE_PX = { row: 280, column: 160 };

/** Pointer listeners for one gesture; Escape cancels it. */
function track(onMove: (event: PointerEvent) => void, onEnd: (apply: boolean) => void) {
  const stop = (apply: boolean) => {
    window.removeEventListener("pointermove", onMove);
    window.removeEventListener("pointerup", up);
    window.removeEventListener("keydown", key);
    onEnd(apply);
  };
  const up = () => stop(true);
  const key = (event: KeyboardEvent) => {
    if (event.key === "Escape") stop(false);
  };
  window.addEventListener("pointermove", onMove);
  window.addEventListener("pointerup", up);
  window.addEventListener("keydown", key);
}

/** The layout of the interactive state and the gestures that change it. */
export function useLiveLayout() {
  const shellRef = useRef<HTMLDivElement>(null);
  const [layout, setLayout] = useState<Layout>({
    tabs: START,
    activeTabId: TAB.tabs.id,
    focusedId: "tabs",
  });
  const [drag, setDrag] = useState<Drag | null>(null);
  const [spare, setSpare] = useState(SPARE);

  const startDrag = (source: DragGhostSource, event: React.PointerEvent<HTMLElement>) => {
    const shell = shellRef.current;
    const sourceTab = source.kind === "tab" ? source.tab : tabOf(layout.tabs, source.windowId);
    if (event.button !== 0 || !shell || !sourceTab) return;
    const origin = { x: event.clientX, y: event.clientY };
    // A window alone in its tab drags as that tab (US-010.EC-1).
    const wholeTab = source.kind === "tab" || leafIds(sourceTab.root).length === 1;
    const effective: DragGhostSource = wholeTab ? { kind: "tab", tab: sourceTab } : source;
    // Dragging the tab on screen shows another tab as the drop target (US-007.EC-1).
    const others = layout.tabs.filter(tab => tab.id !== sourceTab.id);
    const fallback = others.find(tab => !tab.pinned) ?? others[0];
    let contentTabId: string | null = sourceTab.id;
    if (wholeTab) {
      contentTabId =
        sourceTab.id === layout.activeTabId ? (fallback?.id ?? null) : layout.activeTabId;
    }
    let latest: Drag | null = null;
    track(
      moveEvent => {
        const { clientX: x, clientY: y } = moveEvent;
        if (!latest && Math.hypot(x - origin.x, y - origin.y) < 5) return;
        const bounds = shell.getBoundingClientRect();
        const target = resolveDrop(shell, x, y, {
          allowRoot: true,
          skipWindowId: effective.kind === "head" ? effective.windowId : undefined,
        });
        const onBar = target?.kind === "bar";
        latest = {
          source: effective,
          contentTabId: onBar ? null : contentTabId,
          x: x - bounds.left,
          y: y - bounds.top,
          target: !onBar && !contentTabId ? null : target,
        };
        setDrag(latest);
      },
      apply => {
        const finished = latest;
        setDrag(null);
        if (apply && finished) setLayout(current => applyDrop(current, finished));
      }
    );
  };

  const resizeSeam = (splitId: string, index: number, event: React.PointerEvent<HTMLElement>) => {
    const seam = event.currentTarget;
    const before = seam.previousElementSibling;
    const after = seam.nextElementSibling;
    if (!before || !after) return;
    event.preventDefault();
    const row = seam.getAttribute("aria-orientation") === "vertical";
    const size = (element: Element) => {
      const rect = element.getBoundingClientRect();
      return row ? rect.width : rect.height;
    };
    const startSize = size(before);
    const total = startSize + size(after);
    const origin = row ? event.clientX : event.clientY;
    const floor = Math.min(row ? MIN_SHARE_PX.row : MIN_SHARE_PX.column, total / 2);
    const previous = layout;
    track(
      moveEvent => {
        const delta = (row ? moveEvent.clientX : moveEvent.clientY) - origin;
        const next = Math.min(total - floor, Math.max(floor, startSize + delta));
        setLayout(current => resizePair(current, splitId, index, pair => (pair * next) / total));
      },
      apply => {
        if (!apply) setLayout(previous);
      }
    );
  };

  const newTab = () => {
    const windowId = spare[0];
    if (!windowId) return;
    setSpare(spare.slice(1));
    setLayout(current => {
      const at = current.tabs.findIndex(tab => tab.id === current.activeTabId) + 1;
      const tab: DraftTab = { id: `t-live-${windowId}`, root: leaf(windowId) };
      return {
        tabs: [...current.tabs.slice(0, at), tab, ...current.tabs.slice(at)],
        activeTabId: tab.id,
        focusedId: windowId,
      };
    });
  };

  const active = layout.tabs.find(tab => tab.id === layout.activeTabId) ?? layout.tabs[0]!;
  const previewed = drag?.contentTabId
    ? layout.tabs.find(tab => tab.id === drag.contentTabId)
    : undefined;

  return {
    shellRef,
    layout,
    drag,
    contentTab: previewed ?? active,
    startDrag,
    resizeSeam,
    newTab,
    balanceSeam: (splitId: string, index: number) =>
      setLayout(current => resizePair(current, splitId, index, pair => pair / 2)),
    activate: (tabId: string) =>
      setLayout(current => {
        const tab = current.tabs.find(item => item.id === tabId);
        return tab ? { ...current, activeTabId: tabId, focusedId: leafIds(tab.root)[0]! } : current;
      }),
    closeTab: (tabId: string) => setLayout(current => closeTab(current, tabId)),
    closeWindow: (windowId: string) => setLayout(current => closeWindow(current, windowId)),
    focus: (windowId: string) => setLayout(current => ({ ...current, focusedId: windowId })),
    /** "Move to new tab": the window becomes a tab right after its source tab (US-010.AC-1). */
    unsplit: (windowId: string) =>
      setLayout(current => {
        const tab = tabOf(current.tabs, windowId);
        if (!tab) return current;
        return applyDrop(current, {
          source: { kind: "head", windowId },
          contentTabId: null,
          x: 0,
          y: 0,
          target: { kind: "bar", index: current.tabs.indexOf(tab) + 1 },
        });
      }),
  };
}
