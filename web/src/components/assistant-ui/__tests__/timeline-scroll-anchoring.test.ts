import { describe, expect, it } from "vitest";

import { revealTargetInNestedScrollers } from "../hooks/thread-scroll-landing";
import { measureVirtualRow } from "../hooks/use-thread-scroll-controller";

import {
  composerGrowthAdjustment,
  consumeSnapToEnd,
  createVirtualizerMeasurementState,
  FOLD_SETTLE_FRAMES,
  FOLLOW_REARM_BAND_PX,
  getAnchoredTurnMetrics,
  getRowBottom,
  INITIAL_SCROLL_OWNERSHIP,
  measuredComposerOverlay,
  NEW_TURN_ANCHOR_OFFSET_PX,
  nextScrollMode,
  prependScrollTop,
  resolveTimelineIsAtEnd,
  scrollOwnershipTransition,
  shouldMaintainEnd,
  type TimelineListMeasurementState,
  type TimelineScrollMode,
} from "../timeline-scroll-anchoring";

// Suite: timeline scroll anchoring (scroll)
// Invariant: the scroll math and the three-mode transition table follow, anchor,
// and yield to manual navigation on the same numeric contract.
// Boundary IN: pure scroll math + mode reducer + virtualizer measurement shim.
// Boundary OUT: DOM listener wiring + pill rendering, covered by session-thread.test.tsx.

function buildState({
  positions,
  sizes,
  scroll = 0,
  scrollLength = 700,
}: {
  readonly positions: readonly number[];
  readonly sizes: readonly number[];
  readonly scroll?: number;
  readonly scrollLength?: number;
}): TimelineListMeasurementState {
  return {
    data: positions.map((_, index) => index),
    scroll,
    scrollLength,
    positionAtIndex: (index: number) => positions[index],
    sizeAtIndex: (index: number) => sizes[index],
  };
}

describe("timeline scroll anchoring math", () => {
  it("Should measure row bottoms from row position and size", () => {
    const state = buildState({ positions: [0, 120], sizes: [80, 40] });

    expect(getRowBottom(state, 1)).toBe(160);
  });

  it("Should treat the active turn as fitting when it fits above the composer", () => {
    const state = buildState({
      positions: [0, 300, 460],
      sizes: [240, 80, 140],
      scrollLength: 760,
    });

    const metrics = getAnchoredTurnMetrics({
      state,
      anchorIndex: 1,
      composerOverlayHeight: 180,
      anchorOffset: 16,
    });

    expect(metrics?.turnHeight).toBe(300);
    expect(metrics?.usableViewportHeight).toBe(564);
    expect(metrics?.overflowsUsableViewport).toBe(false);
    expect(metrics?.targetScrollToRevealEnd).toBe(36);
    expect(metrics?.scrollDeltaToRevealEnd).toBe(36);
  });

  it("Should target the real row end instead of any temporary reserved tail", () => {
    const state = buildState({
      positions: [0, 1720, 1880],
      sizes: [1600, 80, 120],
      scroll: 1900,
      scrollLength: 760,
    });

    const metrics = getAnchoredTurnMetrics({
      state,
      anchorIndex: 1,
      composerOverlayHeight: 180,
      anchorOffset: 16,
    });

    expect(metrics?.lastBottom).toBe(2000);
    expect(metrics?.targetScrollToRevealEnd).toBe(1436);
    expect(metrics?.scrollDeltaToRevealEnd).toBe(0);
  });

  it("Should report overflow only for the current anchored turn", () => {
    const state = buildState({
      positions: [0, 900, 1180],
      sizes: [800, 220, 300],
      scroll: 900,
      scrollLength: 760,
    });

    const metrics = getAnchoredTurnMetrics({
      state,
      anchorIndex: 1,
      composerOverlayHeight: 180,
      anchorOffset: 16,
    });

    expect(metrics?.turnHeight).toBe(580);
    expect(metrics?.usableViewportHeight).toBe(564);
    expect(metrics?.overflowsUsableViewport).toBe(true);
  });

  it("Should return the minimal positive scroll delta needed to reveal the turn end", () => {
    const state = buildState({
      positions: [0, 900, 1180],
      sizes: [800, 220, 360],
      scroll: 900,
      scrollLength: 760,
    });

    const metrics = getAnchoredTurnMetrics({
      state,
      anchorIndex: 1,
      composerOverlayHeight: 180,
      anchorOffset: 16,
    });

    expect(metrics?.lastBottom).toBe(1540);
    expect(metrics?.visibleUsableBottom).toBe(1464);
    expect(metrics?.scrollDeltaToRevealEnd).toBe(76);
  });

  it("Should subtract composer height from usable viewport height", () => {
    const state = buildState({ positions: [0, 300], sizes: [120, 470], scrollLength: 700 });

    const withoutComposer = getAnchoredTurnMetrics({
      state,
      anchorIndex: 1,
      composerOverlayHeight: 0,
      anchorOffset: 16,
    });
    const withComposer = getAnchoredTurnMetrics({
      state,
      anchorIndex: 1,
      composerOverlayHeight: 220,
      anchorOffset: 16,
    });

    expect(withoutComposer?.overflowsUsableViewport).toBe(false);
    expect(withComposer?.overflowsUsableViewport).toBe(true);
  });

  it("Should return null anchored metrics for an empty list", () => {
    const empty = buildState({ positions: [], sizes: [] });

    expect(
      getAnchoredTurnMetrics({
        state: empty,
        anchorIndex: 0,
        composerOverlayHeight: 0,
        anchorOffset: 16,
      })
    ).toBeNull();
  });
});

describe("timeline scroll mode machine", () => {
  it("Should flip following-end to free-scrolling on manual navigation from any mode", () => {
    // The hook wires wheel, touchmove, and pointerdown to the single
    // manual-navigation event; every mode yields control to the reader.
    const modes: TimelineScrollMode[] = ["following-end", "anchoring-new-turn", "free-scrolling"];
    for (const mode of modes) {
      expect(nextScrollMode(mode, "manual-navigation")).toBe("free-scrolling");
    }
  });

  it("Should enter anchoring-new-turn on a new prompt and pin the user-message metrics", () => {
    expect(nextScrollMode("following-end", "new-turn")).toBe("anchoring-new-turn");
    expect(nextScrollMode("free-scrolling", "new-turn")).toBe("anchoring-new-turn");

    // A freshly-sent turn: user prompt at index 1, assistant answer streaming into
    // index 2. Anchoring on the prompt keeps it near the top and reveals the end.
    const state = buildState({
      positions: [0, 120, 200],
      sizes: [120, 80, 480],
      scroll: 0,
      scrollLength: 700,
    });
    const metrics = getAnchoredTurnMetrics({
      state,
      anchorIndex: 1,
      composerOverlayHeight: 0,
      anchorOffset: 16,
    });

    expect(metrics?.anchorTop).toBe(120);
    expect(metrics?.lastBottom).toBe(680);
    expect(metrics?.overflowsUsableViewport).toBe(false);
  });

  it("Should resume following-end when the live edge is reached or the pill is clicked", () => {
    expect(nextScrollMode("free-scrolling", "reached-end")).toBe("following-end");
    expect(nextScrollMode("free-scrolling", "scroll-to-end")).toBe("following-end");
    expect(nextScrollMode("anchoring-new-turn", "scroll-to-end")).toBe("following-end");
  });
});

describe("virtualizer measurement shim", () => {
  it("Should adapt the virtualizer's measurements to the anchoring math", () => {
    // The shim lets the real hook feed getAnchoredTurnMetrics without the pure math
    // ever importing the virtualizer runtime.
    const shim = createVirtualizerMeasurementState(
      {
        options: { count: 2 },
        scrollOffset: 40,
        measurementsCache: [
          { start: 0, size: 120 },
          { start: 120, size: 80 },
        ],
      },
      500
    );

    expect(shim.data.length).toBe(2);
    expect(shim.scroll).toBe(40);
    expect(shim.scrollLength).toBe(500);
    expect(shim.positionAtIndex(1)).toBe(120);
    expect(getRowBottom(shim, 1)).toBe(200);
  });

  it("Should fall back to zero scroll when the virtualizer has no offset yet", () => {
    const shim = createVirtualizerMeasurementState(
      { options: { count: 1 }, scrollOffset: null, measurementsCache: [{ start: 0, size: 60 }] },
      320
    );

    expect(shim.scroll).toBe(0);
    expect(shim.data.length).toBe(1);
  });
});

// Suite: scroll ownership reducer (ADR-007, US-024, UT-096..UT-098).
// Invariant: one owner at a time — an upward gesture detaches following at once,
// following re-arms only inside the 40px band or through the pill, a fold toggle
// suppresses end-maintenance for two frames, send anchors the new turn against
// the measured composer overlap, steering a streaming turn snaps, older history
// restores the exact offset via the height delta, and composer growth moves the
// viewport only while following.
describe("scroll ownership reducer", () => {
  it("Should detach following instantly on an upward gesture and re-arm only inside the band", () => {
    const free = scrollOwnershipTransition(INITIAL_SCROLL_OWNERSHIP, {
      type: "user-scroll",
      direction: "up",
      atEnd: true,
    });
    expect(free.mode).toBe("free-scrolling");
    // A downward gesture that has not reached the band keeps the reader in control.
    expect(
      scrollOwnershipTransition(free, { type: "user-scroll", direction: "down", atEnd: false }).mode
    ).toBe("free-scrolling");
    expect(
      scrollOwnershipTransition(free, { type: "user-scroll", direction: "down", atEnd: true }).mode
    ).toBe("following-end");
    expect(scrollOwnershipTransition(free, { type: "reached-end" }).mode).toBe("following-end");
    expect(scrollOwnershipTransition(free, { type: "jump-to-latest" }).mode).toBe("following-end");
    expect(FOLLOW_REARM_BAND_PX).toBe(40);
    expect(resolveTimelineIsAtEnd({ contentLength: 1_000, scroll: 560, scrollLength: 400 })).toBe(
      true
    );
    expect(resolveTimelineIsAtEnd({ contentLength: 1_000, scroll: 559, scrollLength: 400 })).toBe(
      false
    );
  });

  it("Should suppress end-maintenance for two frames after a fold toggles", () => {
    const toggled = scrollOwnershipTransition(INITIAL_SCROLL_OWNERSHIP, { type: "fold-toggled" });
    expect(toggled.foldSettleFrames).toBe(FOLD_SETTLE_FRAMES);
    expect(shouldMaintainEnd(toggled)).toBe(false);
    const one = scrollOwnershipTransition(toggled, { type: "frame" });
    expect(shouldMaintainEnd(one)).toBe(false);
    const two = scrollOwnershipTransition(one, { type: "frame" });
    expect(shouldMaintainEnd(two)).toBe(true);
    expect(scrollOwnershipTransition(two, { type: "frame" })).toBe(two);
    // A reader in free mode never gets end-maintenance, frames or not.
    expect(
      shouldMaintainEnd(
        scrollOwnershipTransition(two, { type: "user-scroll", direction: "up", atEnd: false })
      )
    ).toBe(false);
  });

  it("Should anchor a send against the measured composer overlap and snap on a steer during streaming", () => {
    const anchored = scrollOwnershipTransition(INITIAL_SCROLL_OWNERSHIP, { type: "new-turn" });
    expect(anchored.mode).toBe("anchoring-new-turn");
    expect(measuredComposerOverlay({ bottom: 900 }, { top: 860 })).toBe(40);
    expect(measuredComposerOverlay({ bottom: 900 }, { top: 900 })).toBe(0);
    expect(measuredComposerOverlay({ bottom: 900 }, null)).toBe(0);
    const metrics = getAnchoredTurnMetrics({
      state: buildState({ positions: [0, 80, 200], sizes: [80, 120, 600], scrollLength: 640 }),
      anchorIndex: 1,
      composerOverlayHeight: 40,
      anchorOffset: NEW_TURN_ANCHOR_OFFSET_PX,
    });
    expect(metrics?.usableViewportHeight).toBe(640 - 40 - NEW_TURN_ANCHOR_OFFSET_PX);
    expect(scrollOwnershipTransition(anchored, { type: "anchor-outgrown" }).mode).toBe(
      "following-end"
    );

    const steered = scrollOwnershipTransition(anchored, { type: "steer", streaming: true });
    expect(steered).toMatchObject({ mode: "following-end", snapToEnd: true });
    expect(consumeSnapToEnd(steered)).toMatchObject({ mode: "following-end", snapToEnd: false });
    // A steer that lands on an idle turn behaves like a send.
    expect(
      scrollOwnershipTransition(INITIAL_SCROLL_OWNERSHIP, { type: "steer", streaming: false })
    ).toMatchObject({ mode: "anchoring-new-turn", snapToEnd: false });
  });

  it("Should restore the exact offset after a prepend and move for composer growth only while following", () => {
    expect(prependScrollTop({ scrollTop: 320, scrollHeight: 4_000 }, 5_200)).toBe(1_520);
    expect(prependScrollTop({ scrollTop: 0, scrollHeight: 4_000 }, 3_000)).toBe(0);
    expect(composerGrowthAdjustment("following-end", 56)).toBe(56);
    expect(composerGrowthAdjustment("following-end", -20)).toBe(0);
    expect(composerGrowthAdjustment("free-scrolling", 56)).toBe(0);
    expect(composerGrowthAdjustment("anchoring-new-turn", 56)).toBe(0);
  });
});

// Invariant (BUG-20260906-find-specialized-tool-field, long-payload landing): a located
// range inside a nested scrolling box (a payload's bounded body) is first scrolled into that
// box — innermost first, keeping a margin, moving nothing that already shows it — so the
// conversation's outer alignment can measure content that is actually on screen; the
// viewport itself and the row are never treated as nested scrollers.
// Owning layer: scroll landing geometry (`thread-scroll-landing.ts`). Canonical suite: this file.
describe("nested scroller reveal", () => {
  interface Box {
    top: number;
    height: number;
  }

  function rect({ top, height }: Box): DOMRect {
    return {
      bottom: top + height,
      height,
      left: 0,
      right: 100,
      toJSON: () => ({}),
      top,
      width: 100,
      x: 0,
      y: top,
    } as DOMRect;
  }

  /** A viewport > row > payload box (scrolls) > text node; the range sits at `lineTop` inside the box's content. */
  function scene(lineTop: number, boxHeight = 384) {
    const viewport = document.createElement("div");
    const row = document.createElement("div");
    const box = document.createElement("pre");
    const text = document.createTextNode("x".repeat(200));
    box.append(text);
    row.append(box);
    viewport.append(row);
    let scrollTop = 0;
    Object.defineProperty(box, "scrollTop", {
      get: () => scrollTop,
      set: (value: number) => {
        scrollTop = Math.max(0, value);
      },
    });
    Object.defineProperty(box, "scrollHeight", { value: 6_000 });
    Object.defineProperty(box, "clientHeight", { value: boxHeight });
    viewport.getBoundingClientRect = () => rect({ height: 640, top: 0 });
    row.getBoundingClientRect = () => rect({ height: 6_100, top: 100 });
    box.getBoundingClientRect = () => rect({ height: boxHeight, top: 140 });
    const range = new Range();
    range.setStart(text, 10);
    range.setEnd(text, 20);
    range.getBoundingClientRect = () => rect({ height: 14.5, top: 140 + lineTop - scrollTop });
    return { box, range, row, viewport, isScroller: (element: Element) => element === box };
  }

  it("Should scroll the payload box until a line below its bottom edge shows with a margin", () => {
    const { box, range, row, viewport, isScroller } = scene(4_136);
    expect(revealTargetInNestedScrollers(viewport, row, range, isScroller)).toBe(true);
    // 4136 + 14.5 - 384 + 24 = 3790.5: the line rests 24px above the box's bottom.
    expect(box.scrollTop).toBeCloseTo(3_790.5, 5);
    expect(range.getBoundingClientRect().bottom).toBeLessThanOrEqual(140 + 384 - 24);
    expect(range.getBoundingClientRect().top).toBeGreaterThanOrEqual(140 + 24);
  });

  it("Should scroll back up for a line above the box's top edge and leave a visible line alone", () => {
    const above = scene(0);
    above.box.scrollTop = 900;
    expect(
      revealTargetInNestedScrollers(above.viewport, above.row, above.range, above.isScroller)
    ).toBe(true);
    expect(above.box.scrollTop).toBe(0);

    const visible = scene(100);
    expect(
      revealTargetInNestedScrollers(
        visible.viewport,
        visible.row,
        visible.range,
        visible.isScroller
      )
    ).toBe(false);
    expect(visible.box.scrollTop).toBe(0);
  });

  it("Should never scroll the viewport or the row as if they were nested boxes", () => {
    const { range, row, viewport } = scene(4_136);
    expect(revealTargetInNestedScrollers(viewport, row, range, () => true)).toBe(true);
    // Only the box moved: the outer alignment owns the viewport and the row.
    expect(viewport.scrollTop).toBe(0);
    expect(row.scrollTop).toBe(0);
  });
});

// Suite: virtual row measurement (BUG-20260906-promoted-turn-missing-live).
// Invariant: a mounted row that renders nothing measures 0 — never its
// estimate — so the virtual layout equals the real scroll height and the last
// rows stay reachable at the bottom; only an element no layout engine has sized
// keeps the estimate. Owning layer: thread scroll controller (virtualizer).
describe("virtual row measurement", () => {
  const virtualizer = { options: { estimateSize: () => 144 }, indexFromElement: () => 7 };
  const row = () => document.createElement("div");
  const observed = (blockSize: number) =>
    ({ borderBoxSize: [{ blockSize, inlineSize: 800 }] }) as unknown as ResizeObserverEntry;

  it("Should measure an observed empty row as zero, not its estimate", () => {
    expect(measureVirtualRow(row(), observed(0), virtualizer)).toBe(0);
    expect(measureVirtualRow(row(), observed(41.6), virtualizer)).toBe(42);
  });

  it("Should measure a laid-out empty row as zero and an unsized element by its estimate", () => {
    const laidOut = row();
    document.body.append(laidOut);
    try {
      // jsdom has no layout: stand in for a rendered box with no height.
      laidOut.getClientRects = () => [{}] as unknown as DOMRectList;
      expect(measureVirtualRow(laidOut, undefined, virtualizer)).toBe(0);
      const sized = row();
      Object.defineProperty(sized, "offsetHeight", { value: 88 });
      expect(measureVirtualRow(sized, undefined, virtualizer)).toBe(88);
      // No client rects at all: nothing measured this element yet.
      expect(measureVirtualRow(row(), undefined, virtualizer)).toBe(144);
    } finally {
      laidOut.remove();
    }
  });
});
