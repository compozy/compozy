import { useSelector, useStore } from "@xstate/store-react";
import { useEffect, useEffectEvent, type PointerEvent as ReactPointerEvent } from "react";

import type { PixelPoint } from "../lib/window-manager-types";
import { desktopCardDragLogic } from "../stores/desktop-card-drag-store";

/** Controls inside a card that keep their own press: they never start a card drag. */
const NO_CARD_DRAG =
  'input, select, textarea, a, [data-slot="desktop-overview-tab"], [data-slot="delete-button"], button:not([data-slot="desktops-overview-card"])';

export interface DesktopCardDrag {
  desktopId: string;
  /** Where the card would land among the desktops. */
  index: number;
  /** The floating card's top-left and width, relative to the grid. */
  left: number;
  top: number;
  width: number;
}

/** The order with `desktopId` moved to `index`. */
export function movedOrder(order: readonly string[], desktopId: string, index: number): string[] {
  const rest = order.filter(id => id !== desktopId);
  return [...rest.slice(0, index), desktopId, ...rest.slice(index)];
}

/** The click that ends a drag must not switch to the card it was released on. */
function swallowNextClick(): void {
  const swallow = (event: MouseEvent) => {
    event.preventDefault();
    event.stopPropagation();
  };
  window.addEventListener("click", swallow, { capture: true, once: true });
  setTimeout(() => window.removeEventListener("click", swallow, { capture: true }), 0);
}

/** The position in `preview` of the card or gap under `point`, or null over neither. */
function positionUnder(grid: HTMLElement, preview: readonly string[], point: PixelPoint) {
  const previewIndex = new Map(preview.map((id, position) => [id, position]));
  const cards = grid.querySelectorAll<HTMLElement>(
    "[data-slot='desktop-card'], [data-slot='desktop-gap']"
  );
  for (const candidate of cards) {
    const id = candidate.dataset.cardDesktopId;
    const rect = candidate.getBoundingClientRect();
    const over =
      point.x >= rect.left &&
      point.x <= rect.right &&
      point.y >= rect.top &&
      point.y <= rect.bottom;
    const position = id === undefined ? undefined : previewIndex.get(id);
    if (over && position !== undefined) return position;
  }
  return null;
}

/**
 * Drag a desktop card to reorder desktops (US-014.AC-3). The card shows as a gap where it would
 * land while a lifted copy follows the pointer; the release sends one reorder; Escape, a lost
 * pointer, a blurred window or a desktop order that changes under the drag cancel it. The gesture
 * state lives in the card drag store; this hook feeds it pointer and hit-test facts. "Move left" /
 * "Move right" in the card's menu stay the keyboard path.
 */
export function useDesktopCardDrag(
  order: readonly string[],
  onReorder: (desktopId: string, index: number) => void
) {
  const store = useStore(desktopCardDragLogic);
  const state = useSelector(store, snapshot => snapshot.context);
  const drag: DesktopCardDrag | null =
    state.phase === "dragging" && state.desktopId !== null
      ? {
          desktopId: state.desktopId,
          index: state.index,
          left: state.left,
          top: state.top,
          width: state.width,
        }
      : null;
  const previewOrder = drag ? movedOrder(order, drag.desktopId, drag.index) : [...order];

  const reorder = useEffectEvent((desktopId: string, index: number) => onReorder(desktopId, index));
  useEffect(() => {
    const subscription = store.on("reordered", event => reorder(event.desktopId, event.index));
    return () => subscription.unsubscribe();
  }, [store]);

  // The positions a drag measures against are the order it started from: when that order changes
  // (another client) or the overview goes away, the drag ends without sending anything.
  const orderKey = order.join("\n");
  useEffect(() => () => store.trigger.cancelled(), [store, orderKey]);

  const start = (desktopId: string, event: ReactPointerEvent<HTMLElement>) => {
    if (event.button !== 0 || store.getSnapshot().context.phase !== "idle") return;
    if (event.target instanceof Element && event.target.closest(NO_CARD_DRAG)) return;
    const card = event.currentTarget;
    const grid = card.closest<HTMLElement>('[data-slot="desktops-overview-grid"]');
    const from = order.indexOf(desktopId);
    if (grid === null || from < 0) return;
    const cardRect = card.getBoundingClientRect();
    const origin: PixelPoint = { x: event.clientX, y: event.clientY };
    const pointerId = event.pointerId;
    let ended: { unsubscribe: () => void } | null = null;

    const stop = () => {
      window.removeEventListener("pointermove", onMove, true);
      window.removeEventListener("pointerup", onUp, true);
      window.removeEventListener("pointercancel", onCancel, true);
      window.removeEventListener("keydown", onKey, true);
      window.removeEventListener("blur", onCancel);
      ended?.unsubscribe();
    };
    const onMove = (move: PointerEvent) => {
      if (move.pointerId !== pointerId) return;
      const point = { x: move.clientX, y: move.clientY };
      const preview = movedOrder(order, desktopId, store.getSnapshot().context.index);
      const gridRect = grid.getBoundingClientRect();
      store.trigger.moved({
        point,
        grid: { x: gridRect.left, y: gridRect.top },
        overIndex: positionUnder(grid, preview, point),
      });
    };
    const onUp = (up: PointerEvent) => {
      if (up.pointerId !== pointerId) return;
      const dragged = store.getSnapshot().context.phase === "dragging";
      store.trigger.released();
      if (dragged) swallowNextClick();
    };
    const onCancel = () => store.trigger.cancelled();
    const onKey = (key: KeyboardEvent) => {
      if (key.key !== "Escape") return;
      if (store.getSnapshot().context.phase === "dragging") {
        // Escape cancels the card drag only; the overview stays open.
        key.preventDefault();
        key.stopPropagation();
      }
      store.trigger.cancelled();
    };

    window.addEventListener("pointermove", onMove, true);
    window.addEventListener("pointerup", onUp, true);
    window.addEventListener("pointercancel", onCancel, true);
    window.addEventListener("keydown", onKey, true);
    window.addEventListener("blur", onCancel);
    store.trigger.pressed({
      desktopId,
      from,
      origin,
      grab: { x: origin.x - cardRect.left, y: origin.y - cardRect.top },
      width: cardRect.width,
    });
    // However the gesture ends (release, cancel, a changed order), its listeners go with it.
    ended = store.subscribe(snapshot => {
      if (snapshot.context.phase === "idle") stop();
    });
  };

  return { drag, previewOrder, start };
}
