import { useState, type PointerEvent as ReactPointerEvent } from "react";

import type { PixelPoint } from "../lib/window-manager-types";
import { DRAG_START_PX } from "../stores/os-tab-drag-store";

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

/**
 * Drag a desktop card to reorder desktops (US-014.AC-3). The card shows as a gap where it would
 * land while a lifted copy follows the pointer; the release sends one reorder; Escape cancels.
 * "Move left" / "Move right" in the card's menu stay the keyboard path.
 */
export function useDesktopCardDrag(
  order: readonly string[],
  onReorder: (desktopId: string, index: number) => void
) {
  const [drag, setDrag] = useState<DesktopCardDrag | null>(null);
  const previewOrder = drag ? movedOrder(order, drag.desktopId, drag.index) : [...order];

  const start = (desktopId: string, event: ReactPointerEvent<HTMLElement>) => {
    if (event.button !== 0 || drag !== null) return;
    if (event.target instanceof Element && event.target.closest(NO_CARD_DRAG)) return;
    const card = event.currentTarget;
    const grid = card.closest<HTMLElement>('[data-slot="desktops-overview-grid"]');
    const from = order.indexOf(desktopId);
    if (grid === null || from < 0) return;
    const cardRect = card.getBoundingClientRect();
    const origin: PixelPoint = { x: event.clientX, y: event.clientY };
    const grab = { x: origin.x - cardRect.left, y: origin.y - cardRect.top };
    const pointerId = event.pointerId;
    let index = from;
    let dragging = false;

    const update = (point: PixelPoint) => {
      if (!dragging && Math.hypot(point.x - origin.x, point.y - origin.y) < DRAG_START_PX) return;
      dragging = true;
      const preview = movedOrder(order, desktopId, index);
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
        if (id !== undefined && over && preview.includes(id)) {
          index = preview.indexOf(id);
          break;
        }
      }
      const gridRect = grid.getBoundingClientRect();
      setDrag({
        desktopId,
        index,
        left: point.x - gridRect.left - grab.x,
        top: point.y - gridRect.top - grab.y,
        width: cardRect.width,
      });
    };
    const stop = () => {
      window.removeEventListener("pointermove", onMove, true);
      window.removeEventListener("pointerup", onUp, true);
      window.removeEventListener("pointercancel", onCancel, true);
      window.removeEventListener("keydown", onKey, true);
      setDrag(null);
    };
    const onMove = (move: PointerEvent) => {
      if (move.pointerId === pointerId) update({ x: move.clientX, y: move.clientY });
    };
    const onUp = (up: PointerEvent) => {
      if (up.pointerId !== pointerId) return;
      stop();
      if (!dragging) return;
      swallowNextClick();
      if (index === from) return;
      onReorder(desktopId, index);
    };
    const onCancel = () => stop();
    const onKey = (key: KeyboardEvent) => {
      if (key.key !== "Escape") return;
      if (dragging) {
        // Escape cancels the card drag only; the overview stays open.
        key.preventDefault();
        key.stopPropagation();
      }
      stop();
    };
    window.addEventListener("pointermove", onMove, true);
    window.addEventListener("pointerup", onUp, true);
    window.addEventListener("pointercancel", onCancel, true);
    window.addEventListener("keydown", onKey, true);
  };

  return { drag, previewOrder, start };
}
