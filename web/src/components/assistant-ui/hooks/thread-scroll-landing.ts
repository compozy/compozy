/*
 * Row geometry for the scroll owner: which rows reach into the viewport, and
 * the navigation landing (find/trail jumps). The virtualizer's own scrollToFn
 * yields in free mode, so a landing writes the offset directly, frame by
 * frame, until the mounted row — or the located content inside it — rests
 * `offsetTop` under the viewport's top edge. Unmounted rows are approached by
 * the virtualizer's estimate first.
 */

/** Frames a landing waits for the row to mount and settle before giving up. */
const NAVIGATION_LANDING_MAX_FRAMES = 30;
const NAVIGATION_LANDING_STABLE_FRAMES = 2;

export function firstVisibleRow(viewport: HTMLDivElement): HTMLElement | null {
  const viewportTop = viewport.getBoundingClientRect().top;
  const rows = [...viewport.querySelectorAll<HTMLElement>("[data-message-id]")];
  return rows.find(row => row.getBoundingClientRect().bottom >= viewportTop) ?? rows[0] ?? null;
}

export function lastVisibleRow(viewport: HTMLDivElement): HTMLElement | null {
  const viewportBottom = viewport.getBoundingClientRect().bottom;
  const rows = [...viewport.querySelectorAll<HTMLElement>("[data-message-id]")];
  for (let index = rows.length - 1; index >= 0; index -= 1) {
    const row = rows[index]!;
    if (row.getBoundingClientRect().top <= viewportBottom) return row;
  }
  return rows[rows.length - 1] ?? null;
}

function rowForMessage(viewport: HTMLDivElement, messageId: string): HTMLElement | null {
  for (const row of viewport.querySelectorAll<HTMLElement>("[data-message-id]")) {
    if (row.dataset.messageId === messageId) return row;
  }
  return null;
}

/** The element holding a located range (its container, or the container's parent for a text node). */
function targetElement(target: Range): Element | null {
  const container = target.commonAncestorContainer;
  return container instanceof Element ? container : (container.parentElement ?? null);
}

/** The located content's box when the platform can measure a range, else its element's. */
function targetRect(row: HTMLElement, target: Range | null): DOMRect {
  if (target && typeof target.getBoundingClientRect === "function") {
    return target.getBoundingClientRect();
  }
  return ((target && targetElement(target)) ?? row).getBoundingClientRect();
}

/** Room kept between the located content and a nested scroller's edge. */
export const NESTED_REVEAL_MARGIN_PX = 24;

/** A box that scrolls its own overflow (a tool payload's bounded body, a detail rail). */
export function isNestedScroller(element: Element): boolean {
  if (element.scrollHeight <= element.clientHeight + 1) return false;
  const overflowY = getComputedStyle(element).overflowY;
  return overflowY === "auto" || overflowY === "scroll";
}

/**
 * Scrolls every scrolling box between the located content and the viewport so
 * the content sits inside each one (innermost first, since scrolling an inner
 * box moves the content in the outer ones), the way a matched line deep in a
 * bounded payload becomes reachable at all. Only then can the conversation's
 * own alignment place it under the top edge. Returns whether anything moved.
 */
export function revealTargetInNestedScrollers(
  viewport: HTMLElement,
  row: HTMLElement,
  target: Range,
  isScroller: (element: Element) => boolean = isNestedScroller
): boolean {
  let moved = false;
  for (
    let element = targetElement(target);
    element && element !== viewport && element !== row;
    element = element.parentElement
  ) {
    if (!isScroller(element)) continue;
    const rect = targetRect(row, target);
    const box = element.getBoundingClientRect();
    const margin = Math.min(NESTED_REVEAL_MARGIN_PX, Math.max(0, (box.height - rect.height) / 2));
    const before = element.scrollTop;
    if (rect.top < box.top + margin) {
      element.scrollTop = before + (rect.top - box.top) - margin;
    } else if (rect.bottom > box.bottom - margin) {
      element.scrollTop = before + (rect.bottom - box.bottom) + margin;
    }
    if (element.scrollTop !== before) moved = true;
  }
  return moved;
}

export interface LandMessageRowOptions {
  viewport: HTMLDivElement;
  messageId: string;
  offsetTop: number;
  /** The content inside the row to align instead of its top edge, when it exists. */
  locate?: (row: HTMLElement) => Range | null;
  /** The virtualizer's start offset for the row while it is not mounted yet. */
  estimateOffset: () => number | null;
  /** Every programmatic write, so the owner can tell it from the reader's scrolling. */
  onScrolled: (scrollTop: number) => void;
}

/** Resolves `true` once the row rests in place, `false` when it never appeared. */
export function landMessageRow({
  viewport,
  messageId,
  offsetTop,
  locate,
  estimateOffset,
  onScrolled,
}: LandMessageRowOptions): Promise<boolean> {
  return new Promise<boolean>(resolve => {
    let frame = 0;
    let stableFrames = 0;
    const land = () => {
      frame += 1;
      const row = rowForMessage(viewport, messageId);
      const before = viewport.scrollTop;
      let nestedMoved = false;
      if (row) {
        const target = locate?.(row) ?? null;
        // Content inside a bounded box (a payload's scrolling body) is brought
        // into that box first; the outer alignment measures it afterwards.
        if (target) nestedMoved = revealTargetInNestedScrollers(viewport, row, target);
        const top = targetRect(row, target).top;
        const correction = top - viewport.getBoundingClientRect().top - offsetTop;
        if (Math.abs(correction) > 0.5) viewport.scrollTop = Math.max(0, before + correction);
      } else {
        const estimate = estimateOffset();
        if (estimate !== null) viewport.scrollTop = Math.max(0, estimate - offsetTop);
      }
      if (viewport.scrollTop !== before) {
        onScrolled(viewport.scrollTop);
        stableFrames = 0;
      } else if (nestedMoved) {
        stableFrames = 0;
      } else if (row) {
        stableFrames += 1;
      }
      if (row && stableFrames >= NAVIGATION_LANDING_STABLE_FRAMES) {
        resolve(true);
        return;
      }
      if (frame >= NAVIGATION_LANDING_MAX_FRAMES) {
        resolve(row !== null);
        return;
      }
      requestAnimationFrame(land);
    };
    land();
  });
}
