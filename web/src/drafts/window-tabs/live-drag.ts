/** Pointer math for the interactive state: which drop a pointer position means. */
import { nearestEdge, type DropEdge } from "../window-tabs-shared/model";

/** Constants from `_spec.md` Assumptions: root-edge band and tab-drag exit threshold. */
export const ROOT_BAND_PX = 24;
export const EXIT_PX = 24;

export type DropTarget =
  | { kind: "bar"; index: number }
  | { kind: "window"; windowId: string; edge: DropEdge }
  | { kind: "root"; edge: DropEdge }
  | null;

function insertionIndex(bar: Element, x: number): number {
  const tabs = Array.from(bar.querySelectorAll<HTMLElement>('[data-slot="os-tab"]'));
  const index = tabs.findIndex(tab => {
    const rect = tab.getBoundingClientRect();
    return x < rect.left + rect.width / 2;
  });
  return index < 0 ? tabs.length : index;
}

function rootEdge(rect: DOMRect, x: number, y: number): DropEdge | null {
  const distances: [DropEdge, number][] = [
    ["left", x - rect.left],
    ["right", rect.right - x],
    ["top", y - rect.top],
    ["bottom", rect.bottom - y],
  ];
  const [edge, distance] = distances.reduce((best, next) => (next[1] < best[1] ? next : best));
  return distance <= ROOT_BAND_PX ? edge : null;
}

/** Resolves a pointer position inside `shell` to exactly one drop, or none outside both areas. */
export function resolveDrop(
  shell: HTMLElement,
  x: number,
  y: number,
  options: { allowRoot: boolean; skipWindowId?: string }
): DropTarget {
  const bar = shell.querySelector('[data-slot="os-tab-bar"]');
  const content = shell.querySelector('[data-slot="os-split-view"]');
  if (!bar || !content) return null;
  const barRect = bar.getBoundingClientRect();
  if (y < barRect.bottom + EXIT_PX && y >= barRect.top - EXIT_PX) {
    return { kind: "bar", index: insertionIndex(bar, x) };
  }
  const rect = content.getBoundingClientRect();
  if (x < rect.left || x > rect.right || y < rect.top || y > rect.bottom) return null;
  if (options.allowRoot) {
    const edge = rootEdge(rect, x, y);
    if (edge) return { kind: "root", edge };
  }
  const pane = Array.from(content.querySelectorAll<HTMLElement>('[data-slot="os-window"]')).find(
    candidate => {
      const box = candidate.getBoundingClientRect();
      return x >= box.left && x <= box.right && y >= box.top && y <= box.bottom;
    }
  );
  const windowId = pane?.dataset.windowId;
  if (!pane || !windowId || windowId === options.skipWindowId) return null;
  const box = pane.getBoundingClientRect();
  return {
    kind: "window",
    windowId,
    edge: nearestEdge((x - box.left) / box.width, (y - box.top) / box.height),
  };
}
