/**
 * Tab bar width allocation (UT-030). Tabs start at their preferred width and shrink together as
 * the bar narrows; the active tab shrinks at about a third of the rate so it stays the widest.
 * Pinned tabs never shrink. Below the floor the bar stops shrinking and scrolls instead.
 */

/** Widest an unpinned tab gets. */
export const TAB_MAX_WIDTH = 224;
/** Narrowest an unpinned tab gets before the bar overflows. */
export const TAB_MIN_WIDTH = 40;
/** How fast the active tab shrinks relative to the others. */
export const ACTIVE_TAB_SHRINK_RATE = 0.35;

export interface TabWidthInput {
  /** Fixed width of each pinned tab, in bar order. */
  pinnedWidths: readonly number[];
  /** Number of unpinned tabs. */
  unpinnedCount: number;
  /** Index of the active tab among the unpinned tabs, or null when a pinned tab is active. */
  activeIndex: number | null;
  /** Width the strip offers its tabs. */
  available: number;
  /** Space between adjacent tabs. */
  gap?: number;
}

export interface TabWidthAllocation {
  /** Width of each unpinned tab, in bar order. */
  widths: number[];
  /** Tabs at the floor still do not fit: the strip scrolls and the overflow list appears. */
  overflow: boolean;
}

function totalOf(widths: readonly number[]): number {
  return widths.reduce((sum, width) => sum + width, 0);
}

export function allocateTabWidths(input: TabWidthInput): TabWidthAllocation {
  const count = input.unpinnedCount;
  const gap = input.gap ?? 0;
  const tabCount = input.pinnedWidths.length + count;
  const gaps = Math.max(tabCount - 1, 0) * gap;
  const room = Math.max(input.available - totalOf(input.pinnedWidths) - gaps, 0);
  const widths = Array.from({ length: count }, () => TAB_MAX_WIDTH);
  if (count === 0) {
    return { widths, overflow: totalOf(input.pinnedWidths) + gaps > input.available };
  }
  let deficit = totalOf(widths) - room;
  const rate = (index: number) => (index === input.activeIndex ? ACTIVE_TAB_SHRINK_RATE : 1);
  // Flexbox-style shrink: distribute the deficit by rate, freezing tabs that reach the floor.
  while (deficit > 0.001) {
    const shrinking = widths
      .map((width, index) => ({ width, index }))
      .filter(entry => entry.width > TAB_MIN_WIDTH);
    if (shrinking.length === 0) break;
    const totalRate = shrinking.reduce((sum, entry) => sum + rate(entry.index), 0);
    let taken = 0;
    for (const entry of shrinking) {
      const share = (deficit * rate(entry.index)) / totalRate;
      const next = Math.max(TAB_MIN_WIDTH, entry.width - share);
      taken += entry.width - next;
      widths[entry.index] = next;
    }
    deficit -= taken;
  }
  return {
    widths: widths.map(width => Math.floor(width * 100) / 100),
    overflow: deficit > 0.5,
  };
}

/** What a tab of a given width still shows; features drop in this order as it narrows. */
export interface TabPresentation {
  memberCount: boolean;
  waitingCount: boolean;
  signal: boolean;
  close: boolean;
  title: boolean;
}

export function tabPresentation(width: number, active: boolean): TabPresentation {
  if (active) {
    return {
      memberCount: width >= 160,
      waitingCount: width >= 144,
      signal: width >= 120,
      close: width >= 60,
      title: width >= 88,
    };
  }
  return {
    memberCount: width >= 160,
    waitingCount: width >= 144,
    signal: width >= 120,
    close: width >= 120,
    title: width >= 72,
  };
}
