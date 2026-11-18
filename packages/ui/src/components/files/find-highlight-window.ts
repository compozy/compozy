/** A line with at most this many matches paints them all; beyond it, only the visible window. */
export const FIND_DIRECT_PAINT_LIMIT = 64;
/** The most find ranges registered for one paint, across every rendered line. */
export const FIND_PAINT_BUDGET = 300;
/** Characters painted beyond each side of the computed window, absorbing glyph-width drift. */
export const FIND_WINDOW_MARGIN = 64;

export interface FindBox {
  top: number;
  bottom: number;
  left: number;
  right: number;
}

export interface FindLineGeometry {
  /** The rendered line's box (its text starts at `left`). */
  line: FindBox;
  /** The scroll viewport's box. */
  view: FindBox;
  /** Width of one monospace glyph, measured on a short prefix of the line. */
  charWidth: number;
  /** The line's length in UTF-16 units. */
  length: number;
  /** Long lines wrap into rows instead of scrolling sideways. */
  wrap: boolean;
}

/**
 * The half-open character span of a long line that the viewport shows, from geometry alone.
 * Measuring a range deep inside a megabyte-long text node costs time proportional to its offset,
 * so a minified line is never measured match by match (perf audit P-4); the code font is
 * monospace, so an offset maps to a position. The span is widened by `FIND_WINDOW_MARGIN`.
 */
export function visibleCharWindow(geometry: FindLineGeometry): [number, number] {
  const { line, view, charWidth, length, wrap } = geometry;
  if (!(charWidth > 0)) return [0, length];
  let start: number;
  let end: number;
  if (wrap) {
    const perRow = Math.max(1, Math.floor((line.right - line.left) / charWidth));
    const rows = Math.max(1, Math.ceil(length / perRow));
    const rowHeight = (line.bottom - line.top) / rows;
    if (!(rowHeight > 0)) return [0, length];
    const firstRow = Math.floor((view.top - line.top) / rowHeight);
    const lastRow = Math.floor((view.bottom - line.top) / rowHeight);
    start = firstRow * perRow;
    end = (lastRow + 1) * perRow;
  } else {
    start = Math.floor((view.left - line.left) / charWidth);
    end = Math.ceil((view.right - line.left) / charWidth);
  }
  return [
    Math.max(0, start - FIND_WINDOW_MARGIN),
    Math.min(length, Math.max(0, end + FIND_WINDOW_MARGIN)),
  ];
}

/**
 * Positions (in `starts`, ascending match offsets of one line) worth painting: all of them on an
 * ordinary line, else the ones inside `window`, at most `budget`. A binary search finds the first,
 * so a 10,000-match line costs O(log n + budget).
 */
export function matchesInWindow(
  starts: readonly number[],
  window: [number, number] | null,
  budget: number
): number[] {
  if (starts.length === 0 || budget <= 0) return [];
  if (starts.length <= FIND_DIRECT_PAINT_LIMIT || window === null) {
    return Array.from({ length: Math.min(starts.length, budget) }, (_, index) => index);
  }
  const [from, to] = window;
  let low = 0;
  let high = starts.length;
  while (low < high) {
    const middle = (low + high) >>> 1;
    if (starts[middle]! < from) low = middle + 1;
    else high = middle;
  }
  const painted: number[] = [];
  for (let index = low; index < starts.length && painted.length < budget; index += 1) {
    if (starts[index]! >= to) break;
    painted.push(index);
  }
  return painted;
}
