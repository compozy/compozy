// Suite: tab bar width allocation
// Invariant: UT-030 — tabs share the bar between 224px and 40px, the active tab shrinks slowest so
// it stays the widest, pinned tabs keep their width, features drop at their width steps, and the
// bar overflows only once every unpinned tab sits at the floor and still does not fit.
// Boundary IN: pure width allocation and per-width presentation.
// Boundary OUT: rendered tab bar, scrolling, and the overflow list.
import { describe, expect, it } from "vitest";

import { allocateTabWidths, TAB_MAX_WIDTH, TAB_MIN_WIDTH, tabPresentation } from "../tab-width";

function total(widths: readonly number[]): number {
  return widths.reduce((sum, width) => sum + width, 0);
}

describe("allocateTabWidths", () => {
  it("UT-030: Should give every tab the maximum width while the bar has room", () => {
    const allocation = allocateTabWidths({
      pinnedWidths: [],
      unpinnedCount: 3,
      activeIndex: 1,
      available: 1000,
    });

    expect(allocation).toEqual({ widths: [224, 224, 224], overflow: false });
  });

  it("UT-030: Should shrink every tab within bounds and keep the active tab the widest", () => {
    const allocation = allocateTabWidths({
      pinnedWidths: [],
      unpinnedCount: 5,
      activeIndex: 2,
      available: 600,
    });

    expect(allocation.overflow).toBe(false);
    expect(total(allocation.widths)).toBeLessThanOrEqual(600);
    expect(total(allocation.widths)).toBeGreaterThan(599.9);
    for (const width of allocation.widths) {
      expect(width).toBeLessThanOrEqual(TAB_MAX_WIDTH);
      expect(width).toBeGreaterThanOrEqual(TAB_MIN_WIDTH);
    }
    const active = allocation.widths[2] ?? 0;
    allocation.widths.forEach((width, index) => {
      if (index !== 2) expect(active).toBeGreaterThan(width);
    });
  });

  it("UT-030: Should let the active tab absorb the room left once others hit the floor", () => {
    const allocation = allocateTabWidths({
      pinnedWidths: [],
      unpinnedCount: 10,
      activeIndex: 0,
      available: 500,
    });

    expect(allocation.overflow).toBe(false);
    expect(allocation.widths.slice(1)).toEqual(Array.from({ length: 9 }, () => TAB_MIN_WIDTH));
    expect(allocation.widths[0]).toBeCloseTo(140, 1);
  });

  it("UT-030: Should not overflow when every tab exactly fits at the floor", () => {
    const allocation = allocateTabWidths({
      pinnedWidths: [],
      unpinnedCount: 10,
      activeIndex: 4,
      available: 400,
    });

    expect(allocation.overflow).toBe(false);
    expect(allocation.widths).toEqual(Array.from({ length: 10 }, () => TAB_MIN_WIDTH));
  });

  it("UT-030: Should overflow only when tabs at the floor no longer fit", () => {
    const allocation = allocateTabWidths({
      pinnedWidths: [],
      unpinnedCount: 20,
      activeIndex: 0,
      available: 500,
    });

    expect(allocation.overflow).toBe(true);
    expect(allocation.widths).toEqual(Array.from({ length: 20 }, () => TAB_MIN_WIDTH));
  });

  it("UT-030: Should keep pinned widths fixed and share only the remaining room", () => {
    const allocation = allocateTabWidths({
      pinnedWidths: [31, 31],
      unpinnedCount: 3,
      activeIndex: null,
      available: 400,
      gap: 2,
    });

    // 400 - 62 pinned - 4 gaps × 2px leaves 330px for three equal tabs.
    expect(allocation.overflow).toBe(false);
    expect(allocation.widths).toHaveLength(3);
    for (const width of allocation.widths) expect(width).toBeCloseTo(110, 1);
  });

  it("UT-030: Should report overflow when pinned tabs alone exceed the bar", () => {
    expect(
      allocateTabWidths({
        pinnedWidths: [31, 31],
        unpinnedCount: 0,
        activeIndex: null,
        available: 50,
      })
    ).toEqual({ widths: [], overflow: true });
  });
});

describe("tabPresentation", () => {
  it.each([
    [224, { memberCount: true, waitingCount: true, signal: true, close: true, title: true }],
    [159, { memberCount: false, waitingCount: true, signal: true, close: true, title: true }],
    [143, { memberCount: false, waitingCount: false, signal: true, close: true, title: true }],
    [119, { memberCount: false, waitingCount: false, signal: false, close: false, title: true }],
    [71, { memberCount: false, waitingCount: false, signal: false, close: false, title: false }],
  ])("UT-030: Should drop rest-tab features in order at width %s", (width, expected) => {
    expect(tabPresentation(width, false)).toEqual(expected);
  });

  it.each([
    [119, { memberCount: false, waitingCount: false, signal: false, close: true, title: true }],
    [87, { memberCount: false, waitingCount: false, signal: false, close: true, title: false }],
    [60, { memberCount: false, waitingCount: false, signal: false, close: true, title: false }],
    [59, { memberCount: false, waitingCount: false, signal: false, close: false, title: false }],
  ])("UT-030: Should keep the active tab's close button longer at width %s", (width, expected) => {
    expect(tabPresentation(width, true)).toEqual(expected);
  });
});
