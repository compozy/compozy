import { describe, expect, it } from "vitest";

import {
  createVirtualizerMeasurementState,
  getAnchoredTurnMetrics,
  getRowBottom,
  nextScrollMode,
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
